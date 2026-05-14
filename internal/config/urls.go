package config

import (
	"fmt"
	"strings"
)

type URLs struct {
	Authentik  string
	Forgejo    string
	Woodpecker string
	Portainer  string
	Grafana    string
	OpenWebUI  string
	Mattermost string
	Dashboard  string
	DashAlias  string
	OpenClaw   string
	SonarQube  string
	Ghost      string
	Paperclip  string
	CI         string
}

type ServiceLink struct {
	Name string
	URL  string
}

func DeriveURLs(domain string) URLs {
	return URLs{
		Authentik:  fmt.Sprintf("https://auth.%s", domain),
		Forgejo:    fmt.Sprintf("https://git.%s", domain),
		Woodpecker: fmt.Sprintf("https://ci.%s", domain),
		Portainer:  fmt.Sprintf("https://docker.%s", domain),
		Grafana:    fmt.Sprintf("https://grafana.%s", domain),
		OpenWebUI:  fmt.Sprintf("https://ai.%s", domain),
		Mattermost: fmt.Sprintf("https://chat.%s", domain),
		Dashboard:  fmt.Sprintf("https://%s", domain),
		DashAlias:  fmt.Sprintf("https://dash.%s", domain),
		OpenClaw:   fmt.Sprintf("https://openclaw.%s", domain),
		SonarQube:  fmt.Sprintf("https://sonar.%s", domain),
		Ghost:      fmt.Sprintf("https://blog.%s", domain),
		Paperclip:  fmt.Sprintf("https://paperclip.%s", domain),
		CI:         fmt.Sprintf("https://ci.%s", domain),
	}
}

func (u URLs) ServiceLinks() []ServiceLink {
	return []ServiceLink{
		{Name: "Authentik", URL: u.Authentik},
		{Name: "Forgejo", URL: u.Forgejo},
		{Name: "Woodpecker", URL: u.Woodpecker},
		{Name: "Portainer", URL: u.Portainer},
		{Name: "Grafana", URL: u.Grafana},
		{Name: "Open WebUI", URL: u.OpenWebUI},
		{Name: "SonarQube", URL: u.SonarQube},
		{Name: "Mattermost", URL: u.Mattermost},
		{Name: "OpenClaw", URL: u.OpenClaw},
		{Name: "Ghost", URL: u.Ghost},
		{Name: "Paperclip", URL: u.Paperclip},
		{Name: "Homarr", URL: u.Dashboard},
		{Name: "Homarr Alias", URL: u.DashAlias},
	}
}

func (u URLs) Lookup(key string) (string, bool) {
	switch normalizeURLKey(key) {
	case "authentik":
		return u.Authentik, u.Authentik != ""
	case "forgejo":
		return u.Forgejo, u.Forgejo != ""
	case "woodpecker":
		return u.Woodpecker, u.Woodpecker != ""
	case "portainer":
		return u.Portainer, u.Portainer != ""
	case "grafana":
		return u.Grafana, u.Grafana != ""
	case "open_webui":
		return u.OpenWebUI, u.OpenWebUI != ""
	case "mattermost":
		return u.Mattermost, u.Mattermost != ""
	case "dashboard":
		return u.Dashboard, u.Dashboard != ""
	case "dash_alias":
		return u.DashAlias, u.DashAlias != ""
	case "openclaw":
		return u.OpenClaw, u.OpenClaw != ""
	case "sonarqube":
		return u.SonarQube, u.SonarQube != ""
	case "ghost":
		return u.Ghost, u.Ghost != ""
	case "paperclip":
		return u.Paperclip, u.Paperclip != ""
	case "ci":
		return u.CI, u.CI != ""
	default:
		return "", false
	}
}

func normalizeURLKey(key string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(key)), "-", "_")
}
