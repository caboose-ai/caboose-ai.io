package smoketest

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
	"github.com/caboose-ai/caboose-ai.io/internal/secrets"
	"github.com/caboose-ai/caboose-ai.io/internal/services/authentik"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
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

	return &Suite{
		Domain: domain,
		Token:  token,
		AK:     ak,
		URLs:   urls,
		HTTP:   httpClient,
	}
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

	timedPage := page.Timeout(30 * time.Second)
	timedPage.MustWaitStable()

	password := "smoketest-admin-" + fmt.Sprintf("%d", time.Now().Unix())

	el, err := timedPage.ElementByJS(deepQueryOne(`input[type='password']`))
	if err != nil {
		s.ScreenshotOnFailure(t, page)
		t.Fatalf("waiting for password input on recovery page: %v (url: %s)", err, page.MustInfo().URL)
	}
	el.MustInput(password)

	els, err := timedPage.ElementsByJS(deepQueryAll(`input[type='password']`))
	if err == nil && len(els) > 1 {
		els[1].MustInput(password)
	}

	submit, err := timedPage.ElementByJS(deepQueryOne(`button[type='submit']`))
	if err != nil {
		s.ScreenshotOnFailure(t, page)
		t.Fatalf("waiting for submit button on recovery page: %v", err)
	}
	submit.MustClick()
	timedPage.MustWaitStable()

	s.AdminPass = password
	return password
}

func (s *Suite) LoginToAuthentik(t *testing.T, page *rod.Page) {
	t.Helper()

	page.MustWaitStable()

	uid, err := page.ElementByJS(deepQueryOne(`input[name='uidField']`))
	if err != nil {
		t.Fatalf("waiting for uidField: %v", err)
	}
	uid.MustInput("auth-admin")

	submitBtn, err := page.ElementByJS(deepQueryOne(`button[type='submit']`))
	if err != nil {
		t.Fatalf("waiting for submit after uid: %v", err)
	}
	submitBtn.MustClick()
	page.MustWaitStable()

	pw, err := page.ElementByJS(deepQueryOne(`input[name='password']`))
	if err != nil {
		t.Fatalf("waiting for password field: %v", err)
	}
	pw.MustInput(s.AdminPass)

	submitBtn2, err := page.ElementByJS(deepQueryOne(`button[type='submit']`))
	if err != nil {
		t.Fatalf("waiting for submit after password: %v", err)
	}
	submitBtn2.MustClick()
	page.MustWaitStable()
}

func (s *Suite) ScreenshotOnFailure(t *testing.T, page *rod.Page) {
	t.Helper()

	if !t.Failed() {
		return
	}

	dir := filepath.Join(testdataDir(), "failures")
	os.MkdirAll(dir, 0755)

	name := fmt.Sprintf("%s-%d.png", t.Name(), time.Now().Unix())
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
