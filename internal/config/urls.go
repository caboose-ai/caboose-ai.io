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
}

func DeriveURLs(domain string) URLs {
	return URLs{
		Authentik:  fmt.Sprintf("https://auth.%s", domain),
		Forgejo:    fmt.Sprintf("https://git.%s", domain),
		Woodpecker: fmt.Sprintf("https://ci.%s", domain),
		Portainer:  fmt.Sprintf("https://docker.%s", domain),
		Grafana:    fmt.Sprintf("https://grafana.%s", domain),
		OpenWebUI:  fmt.Sprintf("https://chat.%s", domain),
		Mattermost: fmt.Sprintf("https://mattermost.%s", domain),
	}
}
