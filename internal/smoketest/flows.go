package smoketest

import "github.com/caboose-ai/caboose-ai.io/internal/config"

type ServiceFlow struct {
	Name        string
	LoginURL    string
	SSOSelector string
	SSOText     string
	LandingHost string
	PreClick    string
	AutoLogin   bool
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
			Name:        "mattermost",
			LoginURL:    urls.Mattermost + "/login",
			SSOSelector: `a[href*="/oauth/openid"], button[class*="openid"]`,
			SSOText:     "Authentik",
			LandingHost: hostFromURL(urls.Mattermost),
			PreClick:    `a.btn-tertiary`,
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
		{Name: "n8n", URL: urls.N8N, TargetHost: hostFromURL(urls.N8N)},
		{Name: "openclaw", URL: urls.OpenClaw, TargetHost: hostFromURL(urls.OpenClaw)},
	}
}
