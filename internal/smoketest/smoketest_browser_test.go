//go:build integration

package smoketest

import (
	"context"
	"flag"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
)

var smokeFlowName = flag.String("smoke-flow", "", "target one SSO smoke flow by name")

func TestSSO_BrowserFlows(t *testing.T) {
	s := NewSuite(t)
	s.ForceLocalPasswordAuth(t)
	skipIfNoBrowser(t)
	s.InitBrowser(t)
	s.SetupAdminPassword(t)

	t.Run("OAuthFlows", func(t *testing.T) {
		for _, svc := range OAuthServiceFlows(s.URLs) {
			t.Run(svc.Name, func(t *testing.T) {
				testOAuthFlow(t, s, svc)
			})
		}
	})

	t.Run("ProxyFlows", func(t *testing.T) {
		for _, svc := range ProxyFlows(s.URLs) {
			t.Run(svc.Name, func(t *testing.T) {
				testProxyFlow(t, s, svc)
			})
		}
	})
}

func TestSSO_ServiceSmoke(t *testing.T) {
	if *smokeFlowName == "" {
		t.Fatal("-smoke-flow is required")
	}

	s := NewSuite(t)
	s.ForceLocalPasswordAuth(t)
	skipIfNoBrowser(t)
	s.InitBrowser(t)
	s.SetupAdminPassword(t)

	flow, ok := SmokeFlowByName(s.URLs, *smokeFlowName)
	if !ok {
		t.Fatalf("unknown smoke flow %q", *smokeFlowName)
	}
	if flow.OAuth != nil {
		testOAuthFlow(t, s, *flow.OAuth)
		return
	}
	if flow.Proxy != nil {
		testProxyFlow(t, s, *flow.Proxy)
		return
	}
	t.Fatalf("smoke flow %q has no executable target", *smokeFlowName)
}

func TestSSO_LoginPageGoogleOnly(t *testing.T) {
	s := NewSuite(t)
	skipIfNoBrowser(t)
	s.InitBrowser(t)

	page := s.Browser.MustPage("https://openclaw." + s.Domain)
	s.handleDialogs(page)
	defer func() {
		s.ScreenshotOnFailure(t, page)
		page.Close()
	}()

	timedPage := page.Timeout(10 * time.Second)
	s.WaitStable(t, timedPage, "OpenClaw Authentik login page")
	s.Record(t, page, "open", "OpenClaw Authentik login page", map[string]string{"url": page.MustInfo().URL})

	timedPage = page.Timeout(15 * time.Second)
	result, err := timedPage.Eval(`() => {
		function collect(selector) {
			const out = [];
			function walk(root) {
				out.push(...root.querySelectorAll(selector));
				for (const child of root.querySelectorAll('*')) {
					if (child.shadowRoot) walk(child.shadowRoot);
				}
				for (const frame of root.querySelectorAll('iframe')) {
					try {
						if (frame.contentDocument) walk(frame.contentDocument);
					} catch (_) {}
				}
			}
			walk(document);
			return out;
		}
		function visible(el) {
			const style = getComputedStyle(el);
			const rect = el.getBoundingClientRect();
			return style.display !== 'none' && style.visibility !== 'hidden' && rect.width > 0 && rect.height > 0;
		}
		const sourceButtons = collect('button.source-button').filter(visible).map((button) => {
			const style = getComputedStyle(button);
			const img = button.querySelector('img');
			return {
				name: button.getAttribute('name') || '',
				text: button.innerText.trim(),
				ariaLabel: button.getAttribute('aria-label') || '',
				hasIcon: Boolean(img),
				iconSrc: img ? img.getAttribute('src') || '' : '',
				backgroundColor: style.backgroundColor,
				borderColor: style.borderColor,
				color: style.color
			};
		});
		const visibleInputs = collect('input').filter((input) => {
			const type = (input.getAttribute('type') || input.type || '').toLowerCase();
			return visible(input) && !['hidden', 'submit', 'button'].includes(type);
		});
		const submitButtons = collect('button[type="submit"], input[type="submit"]').filter(visible);
		return {
			url: location.href,
			bodyText: document.body.innerText,
			sourceButtonCount: sourceButtons.length,
			sourceButtons,
			visibleInputCount: visibleInputs.length,
			submitButtonCount: submitButtons.length
		};
	}`)
	if err != nil {
		t.Fatalf("reading login page presentation: %v", err)
	}

	state := result.Value
	currentURL := state.Get("url").Str()
	bodyText := state.Get("bodyText").Str()
	if isExternalGoogleLoginURL(currentURL) {
		if !strings.Contains(bodyText, "Sign in with Google") && !strings.Contains(bodyText, "Sign in") {
			t.Fatalf("expected Google-branded sign-in page, got text: %q", bodyText)
		}
		if !strings.Contains(bodyText, "caboose-ai.io") {
			t.Fatalf("expected Google page to continue to caboose-ai.io, got text: %q", bodyText)
		}
		t.Logf("OpenClaw auth handed off to Google sign-in: %s", currentURL)
		return
	}
	if !strings.Contains(currentURL, "auth."+s.Domain) {
		t.Fatalf("expected OpenClaw to redirect to Authentik or Google, got %s", currentURL)
	}
	if got := state.Get("visibleInputCount").Int(); got != 0 {
		t.Fatalf("visible login input count = %d, want 0", got)
	}
	if got := state.Get("submitButtonCount").Int(); got != 0 {
		t.Fatalf("visible submit button count = %d, want 0", got)
	}
	if got := state.Get("sourceButtonCount").Int(); got != 1 {
		t.Fatalf("visible source button count = %d, want 1 (state: %s)", got, state.String())
	}

	button := state.Get("sourceButtons").Arr()[0]
	if name := button.Get("name").Str(); name != "source-Google" {
		t.Fatalf("source button name = %q, want source-Google", name)
	}
	if text := button.Get("text").Str(); text != "Google" {
		t.Fatalf("source button text = %q, want Google", text)
	}
	if !button.Get("hasIcon").Bool() {
		t.Fatalf("Google source button has no icon: %s", button.String())
	}
	if icon := button.Get("iconSrc").Str(); !strings.Contains(icon, "google") {
		t.Fatalf("Google source icon src = %q, want google icon", icon)
	}
	t.Logf("Google-only login page button: %s", button.String())
}

func testOAuthFlow(t *testing.T, s *Suite, svc ServiceFlow) {
	t.Helper()

	basePage := s.Browser.MustPage(svc.LoginURL)
	s.handleDialogs(basePage)
	defer func() {
		s.ScreenshotOnFailure(t, basePage)
		basePage.Close()
	}()

	page := basePage.Timeout(60 * time.Second)
	s.WaitStable(t, page, svc.Name+" login page")
	s.Record(t, page, "open", svc.Name+" login URL", map[string]string{"url": svc.LoginURL})

	if svc.PreClick != "" {
		if pre, err := page.Element(svc.PreClick); err == nil {
			s.Click(t, page, pre, svc.Name+" pre-login control")
			s.WaitStable(t, page, svc.Name+" pre-login transition")
		}
	}

	currentURL := page.MustInfo().URL

	if isExternalGoogleLoginURL(currentURL) {
		t.Skipf("%s handed off to Google login at %s; local password browser smoke cannot complete with email/password login hidden", svc.Name, currentURL)
	}

	if isLoggedInURL(currentURL, svc.LandingHost) {
		t.Logf("%s: already authenticated, at %s", svc.Name, currentURL)
		s.Record(t, page, "landed", svc.Name, map[string]string{"url": currentURL})
		return
	}

	if svc.LocalPasswordSecret != "" {
		loginLocalService(t, s, page, svc)
		finalURL := page.MustInfo().URL
		if !strings.Contains(finalURL, svc.LandingHost) {
			t.Errorf("expected to land on %s, got %s", svc.LandingHost, finalURL)
		}
		if !isLoggedInURL(finalURL, svc.LandingHost) {
			t.Errorf("expected %s to finish logged in, got %s", svc.Name, finalURL)
		}
		s.Record(t, page, "landed", svc.Name, map[string]string{"url": finalURL})
		t.Logf("%s: landed on %s", svc.Name, finalURL)
		return
	}

	if svc.SSOSelector != "" && !strings.Contains(currentURL, "auth."+s.Domain) {
		clicked := false
		if svc.SSOText != "" {
			clicked = s.ClickText(t, page, svc.SSOText, svc.Name+" SSO button")
		}
		if !clicked {
			el, err := page.Element(svc.SSOSelector)
			if err != nil {
				t.Fatalf("SSO button not found for %s (selector: %s): %v", svc.Name, svc.SSOSelector, err)
			}
			s.Click(t, page, el, svc.Name+" SSO button")
		}
		s.WaitStable(t, page, svc.Name+" SSO redirect")
		currentURL = page.MustInfo().URL
		if isExternalGoogleLoginURL(currentURL) {
			t.Skipf("%s handed off to Google login at %s; local password browser smoke cannot complete with email/password login hidden", svc.Name, currentURL)
		}
	}

	if strings.Contains(currentURL, "auth."+s.Domain) {
		s.LoginToAuthentik(t, basePage)
		page = basePage.Timeout(60 * time.Second)
	}

	s.WaitStable(t, page, svc.Name+" landing page")
	finalURL := page.MustInfo().URL
	if svc.Name == "forgejo" && strings.Contains(finalURL, "/user/link_account") {
		linkForgejoAccount(t, s, page)
		finalURL = page.MustInfo().URL
	}

	if !strings.Contains(finalURL, svc.LandingHost) {
		t.Errorf("expected to land on %s, got %s", svc.LandingHost, finalURL)
	}
	if !isLoggedInURL(finalURL, svc.LandingHost) {
		t.Errorf("expected %s to finish logged in, got %s", svc.Name, finalURL)
	}
	s.Record(t, page, "landed", svc.Name, map[string]string{"url": finalURL})
	t.Logf("%s: landed on %s", svc.Name, finalURL)
}

func loginLocalService(t *testing.T, s *Suite, page *rod.Page, svc ServiceFlow) {
	t.Helper()

	if s.ClickText(t, page, "View in Browser", svc.Name+" browser handoff") {
		s.WaitStable(t, page, svc.Name+" browser handoff landing")
		if isLoggedInURL(page.MustInfo().URL, svc.LandingHost) {
			return
		}
	}

	username, err := page.Element(svc.UsernameSelector)
	if err != nil {
		t.Fatalf("%s local username input not found: %v", svc.Name, err)
	}
	password, err := page.Element(svc.PasswordSelector)
	if err != nil {
		t.Fatalf("%s local password input not found: %v", svc.Name, err)
	}
	submit, err := page.Element(svc.SubmitSelector)
	if err != nil {
		t.Fatalf("%s local submit button not found: %v", svc.Name, err)
	}

	s.Input(t, page, username, svc.Name+" local username", svc.LocalUsername, false)
	s.Input(t, page, password, svc.Name+" local password", s.Secret(t, svc.LocalPasswordSecret), true)
	s.Click(t, page, submit, svc.Name+" local login")
	s.WaitStable(t, page, svc.Name+" local landing")
}

func linkForgejoAccount(t *testing.T, s *Suite, page *rod.Page) {
	t.Helper()
	password := s.Secret(t, "GITEA_ADMIN_PASSWORD")

	pw, err := page.Element(`input[name="password"]`)
	if err != nil {
		t.Fatalf("forgejo link account password input not found: %v", err)
	}
	s.Input(t, page, pw, "forgejo local password", password, true)

	submit, err := page.ElementByJS(deepQueryText(`button, input[type="submit"]`, "Link Account"))
	if err != nil {
		submit, err = page.Element(`button[name="submit"], button[type="submit"], input[type="submit"]`)
	}
	if err != nil {
		t.Fatalf("forgejo link account submit not found: %v", err)
	}
	s.Click(t, page, submit, "forgejo link account")
	s.WaitStable(t, page, "forgejo linked account landing")
}

func testProxyFlow(t *testing.T, s *Suite, svc ProxyFlow) {
	t.Helper()

	basePage := s.Browser.MustPage(svc.URL)
	s.handleDialogs(basePage)
	defer func() {
		s.ScreenshotOnFailure(t, basePage)
		basePage.Close()
	}()

	page := basePage.Timeout(60 * time.Second)
	s.WaitStable(t, page, svc.Name+" proxy page")
	s.Record(t, page, "open", svc.Name+" proxy URL", map[string]string{"url": svc.URL})

	currentURL := page.MustInfo().URL

	if isExternalGoogleLoginURL(currentURL) {
		t.Skipf("%s handed off to Google login at %s; local password browser smoke cannot complete with email/password login hidden", svc.Name, currentURL)
	}

	if strings.Contains(currentURL, "auth."+s.Domain) {
		s.LoginToAuthentik(t, basePage)
		page = basePage.Timeout(60 * time.Second)
		s.WaitStable(t, page, svc.Name+" proxy landing page")
		currentURL = page.MustInfo().URL
	}

	if !strings.Contains(currentURL, svc.TargetHost) {
		t.Errorf("expected to reach %s, got %s", svc.TargetHost, currentURL)
	}
	if !isLoggedInURL(currentURL, svc.TargetHost) {
		if svc.AllowLoginLanding && strings.Contains(currentURL, "/login") {
			rejectProxyErrorPage(t, page, svc)
			if svc.HealthURL != "" {
				checkProxyFlowHealth(t, s, svc)
			}
			s.Record(t, page, "reached-upstream-login", svc.Name, map[string]string{"url": currentURL})
			t.Logf("%s: reached upstream login page at %s", svc.Name, currentURL)
			return
		}
		t.Errorf("expected %s to finish logged in, got %s", svc.Name, currentURL)
	}
	rejectProxyErrorPage(t, page, svc)
	if svc.HealthURL != "" {
		checkProxyFlowHealth(t, s, svc)
	}
	s.Record(t, page, "reached", svc.Name, map[string]string{"url": currentURL})
	t.Logf("%s: reached %s", svc.Name, currentURL)
}

func isExternalGoogleLoginURL(rawURL string) bool {
	return strings.Contains(rawURL, "accounts.google.com")
}

func rejectProxyErrorPage(t *testing.T, page *rod.Page, svc ProxyFlow) {
	t.Helper()

	rejectText := append([]string{"bad gateway", "connection refused", "upstream"}, svc.RejectText...)
	body, err := page.Element("body")
	if err != nil {
		return
	}
	text, err := body.Text()
	if err != nil {
		return
	}
	text = strings.ToLower(text)
	for _, reject := range rejectText {
		if strings.Contains(text, reject) {
			t.Fatalf("%s proxy page contains upstream error marker %q", svc.Name, reject)
		}
	}
}

func checkProxyFlowHealth(t *testing.T, s *Suite, svc ProxyFlow) {
	t.Helper()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, svc.HealthURL, nil)
	if err != nil {
		t.Fatalf("%s health request: %v", svc.Name, err)
	}
	resp, err := s.HTTP.Do(req)
	if err != nil {
		t.Fatalf("%s health check failed at %s: %v", svc.Name, svc.HealthURL, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= http.StatusBadRequest {
		t.Fatalf("%s health returned HTTP %d: %s", svc.Name, resp.StatusCode, string(body))
	}
	bodyText := string(body)
	for _, want := range svc.HealthBodyContains {
		if !strings.Contains(bodyText, want) {
			t.Fatalf("%s health body missing %q: %s", svc.Name, want, bodyText)
		}
	}
}
