//go:build integration

package smoketest

import (
	"strings"
	"testing"
	"time"
)

type serviceFlow struct {
	Name        string
	LoginURL    string
	SSOSelector string
	LandingHost string
	PreClick    string
}

func TestSSO_BrowserFlows(t *testing.T) {
	s := NewSuite(t)
	skipIfNoBrowser(t)
	s.InitBrowser(t)
	s.SetupAdminPassword(t)

	services := []serviceFlow{
		{
			Name:        "forgejo",
			LoginURL:    s.URLs.Forgejo + "/user/login",
			SSOSelector: `a[href*="oauth2/Authentik"]`,
			LandingHost: hostFromURL(s.URLs.Forgejo),
		},
		{
			Name:        "grafana",
			LoginURL:    s.URLs.Grafana + "/login",
			SSOSelector: "",
			LandingHost: hostFromURL(s.URLs.Grafana),
		},
		{
			Name:        "portainer",
			LoginURL:    s.URLs.Portainer + "/#!/auth",
			SSOSelector: `button[ng-click*="oAuthLogin"], button[data-cy*="oauth"], a[data-cy*="oauth"]`,
			LandingHost: hostFromURL(s.URLs.Portainer),
		},
		{
			Name:        "mattermost",
			LoginURL:    s.URLs.Mattermost + "/login",
			SSOSelector: `#gitlab, button[class*="gitlab"], a[href*="/oauth/gitlab"]`,
			LandingHost: hostFromURL(s.URLs.Mattermost),
			PreClick:    `a.btn-tertiary`,
		},
	}

	t.Run("OAuthFlows", func(t *testing.T) {
		for _, svc := range services {
			t.Run(svc.Name, func(t *testing.T) {
				testOAuthFlow(t, s, svc)
			})
		}
	})

	t.Run("ProxyFlows", func(t *testing.T) {
		proxyServices := []struct {
			Name       string
			URL        string
			TargetHost string
		}{
			{"dashboard", s.URLs.Dashboard, hostFromURL(s.URLs.Dashboard)},
			{"ci", s.URLs.CI, hostFromURL(s.URLs.CI)},
			{"n8n", s.URLs.N8N, hostFromURL(s.URLs.N8N)},
		}

		for _, svc := range proxyServices {
			t.Run(svc.Name, func(t *testing.T) {
				testProxyFlow(t, s, svc.Name, svc.TargetHost, svc.URL)
			})
		}
	})
}

func testOAuthFlow(t *testing.T, s *Suite, svc serviceFlow) {
	t.Helper()

	basePage := s.Browser.MustPage(svc.LoginURL)
	defer func() {
		s.ScreenshotOnFailure(t, basePage)
		basePage.Close()
	}()

	page := basePage.Timeout(60 * time.Second)
	page.MustWaitStable()

	if svc.PreClick != "" {
		if pre, err := page.Element(svc.PreClick); err == nil {
			pre.MustClick()
			page.MustWaitStable()
		}
	}

	currentURL := page.MustInfo().URL

	if strings.Contains(currentURL, svc.LandingHost) && !strings.Contains(currentURL, "/login") {
		t.Logf("%s: already authenticated, at %s", svc.Name, currentURL)
		return
	}

	if svc.SSOSelector != "" && !strings.Contains(currentURL, "auth."+s.Domain) {
		el, err := page.Element(svc.SSOSelector)
		if err != nil {
			t.Fatalf("SSO button not found for %s (selector: %s): %v", svc.Name, svc.SSOSelector, err)
		}
		el.MustClick()
		page.MustWaitStable()
		currentURL = page.MustInfo().URL
	}

	if strings.Contains(currentURL, "auth."+s.Domain) {
		s.LoginToAuthentik(t, page)
	}

	page.MustWaitStable()
	finalURL := page.MustInfo().URL

	if !strings.Contains(finalURL, svc.LandingHost) {
		t.Errorf("expected to land on %s, got %s", svc.LandingHost, finalURL)
	}
	t.Logf("%s: landed on %s", svc.Name, finalURL)
}

func testProxyFlow(t *testing.T, s *Suite, name, targetHost, url string) {
	t.Helper()

	basePage := s.Browser.MustPage(url)
	defer func() {
		s.ScreenshotOnFailure(t, basePage)
		basePage.Close()
	}()

	page := basePage.Timeout(60 * time.Second)
	page.MustWaitStable()

	currentURL := page.MustInfo().URL

	if strings.Contains(currentURL, "auth."+s.Domain) {
		s.LoginToAuthentik(t, page)
		page.MustWaitStable()
		currentURL = page.MustInfo().URL
	}

	if !strings.Contains(currentURL, targetHost) {
		t.Errorf("expected to reach %s, got %s", targetHost, currentURL)
	}
	t.Logf("%s: reached %s", name, currentURL)
}

func hostFromURL(u string) string {
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	if idx := strings.Index(u, "/"); idx >= 0 {
		u = u[:idx]
	}
	return u
}

// openWebUIFlow handles OpenWebUI's auto-redirect OIDC flow separately
// since it doesn't have a click-to-login button.
func TestSSO_BrowserFlows_OpenWebUI(t *testing.T) {
	s := NewSuite(t)
	skipIfNoBrowser(t)
	s.InitBrowser(t)
	s.SetupAdminPassword(t)

	page := s.Browser.MustPage(s.URLs.OpenWebUI)
	defer func() {
		s.ScreenshotOnFailure(t, page)
		page.MustClose()
	}()

	page.Timeout(30 * time.Second)
	page.MustWaitStable()

	currentURL := page.MustInfo().URL
	if strings.Contains(currentURL, "auth."+s.Domain) {
		s.LoginToAuthentik(t, page)
		page.MustWaitStable()
	}

	finalURL := page.MustInfo().URL
	expected := hostFromURL(s.URLs.OpenWebUI)
	if !strings.Contains(finalURL, expected) {
		t.Errorf("expected to land on %s, got %s", expected, finalURL)
	}
	t.Logf("open-webui: landed on %s", finalURL)
}
