package smoketest

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
	"github.com/caboose-ai/caboose-ai.io/internal/docker"
	"github.com/caboose-ai/caboose-ai.io/internal/runner"
	"github.com/caboose-ai/caboose-ai.io/internal/secrets"
	"github.com/caboose-ai/caboose-ai.io/internal/services/authentik"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

const defaultDomain = "caboose-ai.io"

type Suite struct {
	Domain    string
	Token     string
	AK        *authentik.Client
	URLs      config.URLs
	HTTP      *http.Client
	Browser   *rod.Browser
	AdminPass string
	Evidence  *EvidenceRecorder
}

func NewSuite(t *testing.T) *Suite {
	t.Helper()

	domain := os.Getenv("SMOKETEST_DOMAIN")
	if domain == "" {
		domain = defaultDomain
	}

	token := os.Getenv("AUTHENTIK_TOKEN")
	if token == "" {
		token = os.Getenv("AUTHENTIK_BOOTSTRAP_TOKEN")
	}
	if token == "" {
		envPath := findEnvFile()
		if envPath != "" {
			store := secrets.NewEnvFileStore(envPath)
			val, err := store.Get(context.Background(), "AUTHENTIK_BOOTSTRAP_TOKEN")
			if err == nil && val != "" {
				token = val
			}
		}
	}

	if token == "" {
		t.Skip("AUTHENTIK_TOKEN or AUTHENTIK_BOOTSTRAP_TOKEN not set (checked env and .env)")
	}

	urls := config.DeriveURLs(domain)

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	ak := authentik.NewClient(urls.Authentik, token, httpClient)
	if !authentikTokenWorks(context.Background(), ak) {
		recovered, err := recoverAuthentikTokenFromContainer(context.Background())
		if err != nil {
			t.Fatalf("AUTHENTIK_TOKEN is invalid and token recovery failed: %v", err)
		}
		token = recovered
		ak = authentik.NewClient(urls.Authentik, token, httpClient)
	}

	return &Suite{
		Domain: domain,
		Token:  token,
		AK:     ak,
		URLs:   urls,
		HTTP:   httpClient,
	}
}

func authentikTokenWorks(ctx context.Context, ak *authentik.Client) bool {
	_, err := ak.Get(ctx, "/api/v3/admin/version/")
	return err == nil
}

func recoverAuthentikTokenFromContainer(ctx context.Context) (string, error) {
	type recoveryResult struct {
		Token string `json:"token"`
		Error string `json:"error"`
	}

	script := strings.Join([]string{
		"import json",
		"import traceback",
		"try:",
		"    from authentik.core.models import Group, Token, TokenIntents, User, UserTypes, default_token_key",
		"    u = User.objects.filter(username='auth-admin').first() or User.objects.filter(username='akadmin').first()",
		"    if u is None:",
		"        u = User.objects.create(username='auth-admin', name='authentik Default Admin', type=UserTypes.INTERNAL)",
		"    u.is_active = True",
		"    u.save()",
		"    g, _ = Group.objects.get_or_create(name='authentik Admins', defaults={'is_superuser': True})",
		"    if not g.is_superuser:",
		"        g.is_superuser = True",
		"        g.save(update_fields=['is_superuser'])",
		"    g.users.add(u)",
		"    t, _ = Token.objects.update_or_create(identifier='authentik-bootstrap-token', defaults={'intent': TokenIntents.INTENT_API, 'user': u, 'expiring': False, 'key': default_token_key()})",
		"    print(json.dumps({'token': t.key}))",
		"except Exception as exc:",
		"    print(json.dumps({'error': ''.join(traceback.format_exception_only(type(exc), exc)).strip()}))",
	}, "\n")

	execClient := docker.NewExecClient(runner.NewLocalRunner())
	out, err := execClient.Exec(ctx, "authentik-server", "sh", "-c", "PATH=/ak-root/.venv/bin:$PATH /lifecycle/ak shell -c \"$1\"", "homelab-smoketest-recover-token", script)
	if err != nil {
		return "", fmt.Errorf("creating Authentik bootstrap token in container: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	rawResult := strings.TrimSpace(lines[len(lines)-1])
	var result recoveryResult
	if err := json.Unmarshal([]byte(rawResult), &result); err != nil {
		return "", fmt.Errorf("unexpected recovery output")
	}
	if result.Error != "" {
		return "", errors.New(result.Error)
	}
	if result.Token == "" {
		return "", fmt.Errorf("empty token")
	}
	return result.Token, nil
}

const (
	turnstileTestSiteKey = "1x00000000000000000000AA"
	turnstileTestSecret  = "1x0000000000000000000000000000000AA"
)

func (s *Suite) InitBrowser(t *testing.T) {
	t.Helper()

	chromePath := findChromium()
	if chromePath == "" {
		t.Skip("Chromium binary not found (checked ~/.cache/ms-playwright/)")
	}

	s.swapCaptchaToTestKeys(t)

	headed := os.Getenv("SMOKETEST_HEADED") == "1"

	l := launcher.New().
		Bin(chromePath).
		Headless(!headed).
		Set("no-sandbox").
		Set("disable-gpu")

	url := l.MustLaunch()

	browser := rod.New().ControlURL(url).MustConnect()
	browser.MustIgnoreCertErrors(true)

	s.Browser = browser
	s.Evidence = NewEvidenceRecorder(t)

	t.Cleanup(func() {
		browser.MustClose()
	})
}

func (s *Suite) swapCaptchaToTestKeys(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	stage, err := s.AK.GetCaptchaStage(ctx, "turnstile-captcha")
	if err != nil || stage == nil {
		return
	}

	origSecret := os.Getenv("TURNSTILE_SECRET_KEY")
	if origSecret == "" {
		envPath := findEnvFile()
		if envPath != "" {
			store := secrets.NewEnvFileStore(envPath)
			origSecret, _ = store.Get(ctx, "TURNSTILE_SECRET_KEY")
		}
	}
	if origSecret == "" {
		store := secrets.NewSplitOnePasswordStore("Homelab - Static", "Homelab - Dynamic", runner.NewLocalRunner(), findEnvFile())
		origSecret, _ = store.Get(ctx, "TURNSTILE_SECRET_KEY")
	}

	err = s.AK.PatchCaptchaStage(ctx, stage.PK, authentik.CreateCaptchaStageParams{
		Name:       "turnstile-captcha",
		PublicKey:  turnstileTestSiteKey,
		PrivateKey: turnstileTestSecret,
		JsURL:      "https://challenges.cloudflare.com/turnstile/v0/api.js",
		ApiURL:     "https://challenges.cloudflare.com/turnstile/v0/siteverify",
	})
	if err != nil {
		t.Logf("warning: could not swap captcha to test keys: %v", err)
		return
	}
	t.Logf("Swapped captcha stage to Turnstile test keys")

	t.Cleanup(func() {
		if origSecret == "" {
			t.Logf("warning: TURNSTILE_SECRET_KEY not available, cannot restore production captcha keys")
			return
		}
		origSiteKey := os.Getenv("TURNSTILE_SITE_KEY")
		if origSiteKey == "" {
			envPath := findEnvFile()
			if envPath != "" {
				store := secrets.NewEnvFileStore(envPath)
				origSiteKey, _ = store.Get(context.Background(), "TURNSTILE_SITE_KEY")
			}
		}
		if origSiteKey == "" {
			store := secrets.NewSplitOnePasswordStore("Homelab - Static", "Homelab - Dynamic", runner.NewLocalRunner(), findEnvFile())
			origSiteKey, _ = store.Get(context.Background(), "TURNSTILE_SITE_KEY")
		}
		if origSiteKey == "" {
			t.Logf("warning: TURNSTILE_SITE_KEY not available, cannot restore production captcha keys")
			return
		}
		err := s.AK.PatchCaptchaStage(context.Background(), stage.PK, authentik.CreateCaptchaStageParams{
			Name:       "turnstile-captcha",
			PublicKey:  origSiteKey,
			PrivateKey: origSecret,
			JsURL:      "https://challenges.cloudflare.com/turnstile/v0/api.js",
			ApiURL:     "https://challenges.cloudflare.com/turnstile/v0/siteverify",
		})
		if err != nil {
			t.Logf("warning: could not restore captcha keys: %v", err)
		} else {
			t.Logf("Restored captcha stage to production keys")
		}
	})
}

func (s *Suite) SetupAdminPassword(t *testing.T) string {
	t.Helper()

	ctx := context.Background()

	user, err := s.AK.FindUser(ctx, "auth-admin")
	if err != nil {
		t.Fatalf("finding auth-admin user: %v", err)
	}
	if user == nil {
		t.Fatal("auth-admin user not found")
	}

	link, err := s.AK.GenerateRecoveryLink(ctx, user.PK)
	if err != nil {
		t.Fatalf("generating recovery link: %v", err)
	}

	page := s.Browser.MustPage(link)
	defer page.Close()
	s.Record(t, page, "open", "admin recovery link", nil)

	timedPage := page.Timeout(30 * time.Second)
	s.WaitStable(t, timedPage, "admin recovery page")
	s.Record(t, page, "wait", "admin recovery page stable", nil)

	password := "smoketest-admin-" + fmt.Sprintf("%d", time.Now().Unix())

	el, err := timedPage.ElementByJS(deepQueryOne(`input[type='password']`))
	if err != nil {
		s.ScreenshotOnFailure(t, page)
		t.Fatalf("waiting for password input on recovery page: %v (url: %s)", err, page.MustInfo().URL)
	}
	s.Input(t, page, el, "new admin password", password, true)

	els, err := timedPage.ElementsByJS(deepQueryAll(`input[type='password']`))
	if err == nil && len(els) > 1 {
		s.Input(t, page, els[1], "confirm admin password", password, true)
	}

	submit, err := timedPage.ElementByJS(deepQueryOne(`button[type='submit']`))
	if err != nil {
		s.ScreenshotOnFailure(t, page)
		t.Fatalf("waiting for submit button on recovery page: %v", err)
	}
	s.Click(t, page, submit, "submit admin password reset")
	s.WaitStable(t, timedPage, "admin password reset")
	s.Record(t, page, "wait", "admin password reset complete", nil)

	s.AdminPass = password
	return password
}

func (s *Suite) LoginToAuthentik(t *testing.T, page *rod.Page) {
	t.Helper()

	s.WaitStable(t, page, "authentik login page")

	uid, err := page.ElementByJS(deepQueryOne(`input[name='uidField']`))
	if err != nil {
		t.Fatalf("waiting for uidField: %v", err)
	}
	s.Input(t, page, uid, "authentik username", "auth-admin", false)

	submitBtn, err := page.ElementByJS(deepQueryOne(`button[type='submit']`))
	if err != nil {
		t.Fatalf("waiting for submit after uid: %v", err)
	}
	s.Click(t, page, submitBtn, "submit username")
	s.WaitStable(t, page, "authentik password stage")

	pw, err := page.ElementByJS(deepQueryOne(`input[name='password']`))
	if err != nil {
		t.Fatalf("waiting for password field: %v", err)
	}
	s.Input(t, page, pw, "authentik password", s.AdminPass, true)

	submitBtn2, err := page.ElementByJS(deepQueryOne(`button[type='submit']`))
	if err != nil {
		t.Fatalf("waiting for submit after password: %v", err)
	}
	s.Click(t, page, submitBtn2, "submit password")
	s.WaitStable(t, page, "authentik login completion")
	s.Record(t, page, "wait", "authentik login complete", nil)
}

func (s *Suite) WaitStable(t *testing.T, page *rod.Page, label string) {
	t.Helper()
	if err := page.WaitStable(time.Second); err != nil {
		s.ScreenshotOnFailure(t, page)
		t.Fatalf("waiting for %s to stabilize: %v", label, err)
	}
}

func (s *Suite) Record(t *testing.T, page *rod.Page, action, label string, details map[string]string) {
	t.Helper()
	if s.Evidence != nil {
		s.Evidence.Record(t, page, action, label, details)
	}
}

func (s *Suite) Click(t *testing.T, page *rod.Page, el *rod.Element, label string) {
	t.Helper()
	s.Record(t, page, "before_click", label, nil)
	if err := el.Click(proto.InputMouseButtonLeft, 1); err != nil {
		s.ScreenshotOnFailure(t, page)
		t.Fatalf("clicking %s: %v", label, err)
	}
	s.Record(t, page, "after_click", label, nil)
}

func (s *Suite) ClickText(t *testing.T, page *rod.Page, text, label string) bool {
	t.Helper()
	el, err := page.ElementByJS(deepQueryText(`button, a`, text))
	if err != nil {
		return false
	}
	s.Click(t, page, el, label)
	return true
}

func (s *Suite) Input(t *testing.T, page *rod.Page, el *rod.Element, label, value string, secret bool) {
	t.Helper()
	details := map[string]string{"value": value}
	if secret {
		details["value"] = "[REDACTED]"
	}
	s.Record(t, page, "before_input", label, details)
	if err := el.Input(value); err != nil {
		s.ScreenshotOnFailure(t, page)
		t.Fatalf("entering %s: %v", label, err)
	}
	s.Record(t, page, "after_input", label, details)
}

func (s *Suite) Secret(t *testing.T, key string) string {
	t.Helper()
	store := secrets.NewSplitOnePasswordStore("Homelab - Static", "Homelab - Dynamic", runner.NewLocalRunner(), findEnvFile())
	value, err := store.Get(context.Background(), key)
	if err != nil || value == "" {
		t.Fatalf("reading secret %s: %v", key, err)
	}
	return value
}

func (s *Suite) ScreenshotOnFailure(t *testing.T, page *rod.Page) {
	t.Helper()

	if !t.Failed() {
		return
	}

	dir := filepath.Join(testdataDir(), "failures")
	os.MkdirAll(dir, 0755)

	name := fmt.Sprintf("%s-%d.png", safeName(t.Name()), time.Now().Unix())
	path := filepath.Join(dir, name)
	data := page.MustScreenshot()
	os.WriteFile(path, data, 0644)
	t.Logf("Screenshot saved: %s", path)
}

func findEnvFile() string {
	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Dir(thisFile)
	for i := 0; i < 5; i++ {
		candidate := filepath.Join(dir, ".env")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		dir = filepath.Dir(dir)
	}
	return ""
}

func findChromium() string {
	home, _ := os.UserHomeDir()
	patterns := []string{
		filepath.Join(home, ".cache/ms-playwright/chromium-*/chrome-linux64/chrome"),
		filepath.Join(home, ".cache/ms-playwright/chromium-*/chrome-linux/chrome"),
	}
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(pattern)
		if len(matches) > 0 {
			return matches[0]
		}
	}
	return ""
}

func testdataDir() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "testdata")
}

func skipIfNoBrowser(t *testing.T) {
	t.Helper()
	if findChromium() == "" {
		t.Skip("Chromium not found")
	}
}

// Retry-aware HTTP GET that follows the suite's TLS and redirect config.
func (s *Suite) get(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return s.HTTP.Do(req)
}

func (s *Suite) getWithAuth(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.Token)
	return s.HTTP.Do(req)
}

func deepQueryOne(selector string) *rod.EvalOptions {
	return rod.Eval(`(selector) => {
		function deepQuery(root, sel) {
			let el = root.querySelector(sel);
			if (el) return el;
			for (const child of root.querySelectorAll('*')) {
				if (child.shadowRoot) {
					el = deepQuery(child.shadowRoot, sel);
					if (el) return el;
				}
			}
			return null;
		}
		return deepQuery(document, selector);
	}`, selector)
}

func deepQueryAll(selector string) *rod.EvalOptions {
	return rod.Eval(`(selector) => {
		const results = [];
		function deepQueryAll(root, sel) {
			results.push(...root.querySelectorAll(sel));
			for (const child of root.querySelectorAll('*')) {
				if (child.shadowRoot) {
					deepQueryAll(child.shadowRoot, sel);
				}
			}
		}
		deepQueryAll(document, selector);
		return results;
	}`, selector)
}

func deepQueryText(selector, text string) *rod.EvalOptions {
	return rod.Eval(`(selector, text) => {
		function visibleText(el) {
			return (el.innerText || el.textContent || '').trim();
		}
		function deepQuery(root, sel, needle) {
			for (const el of root.querySelectorAll(sel)) {
				if (visibleText(el).includes(needle)) return el;
			}
			for (const child of root.querySelectorAll('*')) {
				if (child.shadowRoot) {
					const el = deepQuery(child.shadowRoot, sel, needle);
					if (el) return el;
				}
			}
			return null;
		}
		return deepQuery(document, selector, text);
	}`, selector, text)
}
