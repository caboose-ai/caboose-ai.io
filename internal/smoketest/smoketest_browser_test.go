//go:build integration

package smoketest

import (
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
)

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
				testProxyFlow(t, s, svc.Name, svc.TargetHost, svc.URL)
			})
		}
	})
}

func testOAuthFlow(t *testing.T, s *Suite, svc ServiceFlow) {
	t.Helper()

	basePage := s.Browser.MustPage(svc.LoginURL)
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

	if svc.SSOSelector != "" && !strings.Contains(currentURL, "auth."+s.Domain) {
		el, err := page.Element(svc.SSOSelector)
		if err != nil && svc.SSOText != "" {
			if s.ClickText(t, page, svc.SSOText, svc.Name+" SSO button") {
				err = nil
			}
		}
		if err != nil {
			t.Fatalf("SSO button not found for %s (selector: %s): %v", svc.Name, svc.SSOSelector, err)
		}
		if el != nil {
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

func linkForgejoAccount(t *testing.T, s *Suite, page *rod.Page) {
	t.Helper()
	password := s.Secret(t, "GITEA_ADMIN_PASSWORD")

	pw, err := page.Element(`input[name="password"]`)
	if err != nil {
		t.Fatalf("forgejo link account password input not found: %v", err)
	}
	s.Input(t, page, pw, "forgejo local password", password, true)

	submit, err := page.Element(`button[type="submit"]`)
	if err != nil {
		t.Fatalf("forgejo link account submit not found: %v", err)
	}
	s.Click(t, page, submit, "forgejo link account")
	s.WaitStable(t, page, "forgejo linked account landing")
}

func testProxyFlow(t *testing.T, s *Suite, name, targetHost, url string) {
	t.Helper()

	basePage := s.Browser.MustPage(url)
	defer func() {
		s.ScreenshotOnFailure(t, basePage)
		basePage.Close()
	}()

	page := basePage.Timeout(60 * time.Second)
	s.WaitStable(t, page, name+" proxy page")
	s.Record(t, page, "open", name+" proxy URL", map[string]string{"url": url})

	currentURL := page.MustInfo().URL

	if strings.Contains(currentURL, "auth."+s.Domain) {
		s.LoginToAuthentik(t, page)
		s.WaitStable(t, page, name+" proxy landing page")
		currentURL = page.MustInfo().URL
	}

	if !strings.Contains(currentURL, targetHost) {
		t.Errorf("expected to reach %s, got %s", targetHost, currentURL)
	}
	s.Record(t, page, "reached", name, map[string]string{"url": currentURL})
	t.Logf("%s: reached %s", name, currentURL)
}

func isLoggedInURL(currentURL, host string) bool {
	if !strings.Contains(currentURL, host) {
		return false
	}
	failMarkers := []string{
		"/login",
		"/auth",
		"/error",
		"/user/link_account",
		"#!/auth",
	}
	for _, marker := range failMarkers {
		if strings.Contains(currentURL, marker) {
			return false
		}
	}
	return true
}
