package config

import "fmt"

type URLs struct {
	Authentik   string
	Forgejo     string
	Woodpecker  string
	Portainer   string
	Grafana     string
	OpenWebUI   string
	Mattermost  string
	Dashboard   string
	DashAlias   string
	N8N         string
	OpenClaw    string
	CI          string
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
		N8N:        fmt.Sprintf("https://n8n.%s", domain),
		OpenClaw:   fmt.Sprintf("https://openclaw.%s", domain),
		CI:         fmt.Sprintf("https://ci.%s", domain),
	}
}
