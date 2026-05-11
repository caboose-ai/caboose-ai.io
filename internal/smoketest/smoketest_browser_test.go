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
	}

	if strings.Contains(currentURL, "auth."+s.Domain) {
		s.LoginToAuthentik(t, page)
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

	if strings.Contains(currentURL, "auth."+s.Domain) {
		s.LoginToAuthentik(t, page)
		s.WaitStable(t, page, svc.Name+" proxy landing page")
		currentURL = page.MustInfo().URL
	}

	if !strings.Contains(currentURL, svc.TargetHost) {
		t.Errorf("expected to reach %s, got %s", svc.TargetHost, currentURL)
	}
	if !isLoggedInURL(currentURL, svc.TargetHost) {
		t.Errorf("expected %s to finish logged in, got %s", svc.Name, currentURL)
	}
	rejectProxyErrorPage(t, page, svc)
	if svc.HealthURL != "" {
		checkProxyFlowHealth(t, s, svc)
	}
	s.Record(t, page, "reached", svc.Name, map[string]string{"url": currentURL})
	t.Logf("%s: reached %s", svc.Name, currentURL)
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
