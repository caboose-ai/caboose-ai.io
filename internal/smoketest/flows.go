package smoketest

import "github.com/caboose-ai/caboose-ai.io/internal/config"

type ServiceFlow struct {
	Name                string
	LoginURL            string
	SSOSelector         string
	SSOText             string
	LandingHost         string
	PreClick            string
	AutoLogin           bool
	LocalUsername       string
	LocalPasswordSecret string
	UsernameSelector    string
	PasswordSelector    string
	SubmitSelector      string
}

type ProxyFlow struct {
	Name       string
	URL        string
	TargetHost string
}

func OAuthServiceFlows(urls config.URLs) []ServiceFlow {
	return []ServiceFlow{
		{
			Name:        "forgejo",
			LoginURL:    urls.Forgejo + "/user/login",
			SSOSelector: `a[href*="oauth2/Authentik"]`,
			LandingHost: hostFromURL(urls.Forgejo),
		},
		{
			Name:        "grafana",
			LoginURL:    urls.Grafana + "/login",
			LandingHost: hostFromURL(urls.Grafana),
			AutoLogin:   true,
		},
		{
			Name:        "portainer",
			LoginURL:    urls.Portainer + "/#!/auth",
			SSOSelector: `button[ng-click*="oAuthLogin"], button[data-cy*="oauth"], a[data-cy*="oauth"]`,
			SSOText:     "OAuth",
			LandingHost: hostFromURL(urls.Portainer),
		},
		{
			Name:                "mattermost",
			LoginURL:            urls.Mattermost + "/login",
			LandingHost:         hostFromURL(urls.Mattermost),
			LocalUsername:       "auth-admin",
			LocalPasswordSecret: "MATTERMOST_ADMIN_PASSWORD",
			UsernameSelector:    `#input_loginId`,
			PasswordSelector:    `#input_password-input`,
			SubmitSelector:      `#saveSetting`,
		},
		{
			Name:        "open-webui",
			LoginURL:    urls.OpenWebUI + "/oauth/oidc/login",
			LandingHost: hostFromURL(urls.OpenWebUI),
		},
		{
			Name:        "homarr",
			LoginURL:    urls.Dashboard,
			LandingHost: hostFromURL(urls.Dashboard),
			AutoLogin:   true,
		},
	}
}

func ProxyFlows(urls config.URLs) []ProxyFlow {
	return []ProxyFlow{
		{Name: "dashboard", URL: urls.Dashboard, TargetHost: hostFromURL(urls.Dashboard)},
		{Name: "dashboard-alias", URL: urls.DashAlias, TargetHost: hostFromURL(urls.Dashboard)},
		{Name: "ci", URL: urls.CI, TargetHost: hostFromURL(urls.CI)},
		{Name: "openclaw", URL: urls.OpenClaw, TargetHost: hostFromURL(urls.OpenClaw)},
	}
}
