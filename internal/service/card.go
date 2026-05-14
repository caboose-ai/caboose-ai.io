package service

import "github.com/caboose-ai/caboose-ai.io/internal/config"

type Card struct {
	Slug            string        `json:"slug"`
	DisplayName     string        `json:"display_name"`
	Runtime         Runtime       `json:"runtime"`
	URLKey          string        `json:"url_key"`
	ResolvedURL     string        `json:"resolved_url,omitempty"`
	ComposeServices []string      `json:"compose_services"`
	SecretNames     []string      `json:"secret_names"`
	Configurator    string        `json:"configurator,omitempty"`
	SmokeFlow       string        `json:"smoke_flow,omitempty"`
	DashboardPolicy DashboardSpec `json:"dashboard_policy"`
	SSOMode         string        `json:"sso_mode,omitempty"`
	Health          HealthSpec    `json:"health"`
	Docs            []string      `json:"docs"`
}

func BuildCards(registry *Registry, urls config.URLs) []Card {
	if registry == nil {
		return nil
	}

	slugs := registry.Slugs()
	cards := make([]Card, 0, len(slugs))
	for _, slug := range slugs {
		manifest, ok := registry.Manifest(slug)
		if !ok {
			continue
		}
		cards = append(cards, BuildCard(manifest, urls))
	}
	return cards
}

func BuildCard(manifest Manifest, urls config.URLs) Card {
	resolvedURL := ""
	if manifest.URLKey != "" {
		if url, ok := urls.Lookup(manifest.URLKey); ok {
			resolvedURL = url
		}
	}

	return Card{
		Slug:            manifest.Slug,
		DisplayName:     manifest.DisplayName,
		Runtime:         manifest.Runtime,
		URLKey:          manifest.URLKey,
		ResolvedURL:     resolvedURL,
		ComposeServices: append([]string(nil), manifest.ComposeServices...),
		SecretNames:     append([]string(nil), manifest.Secrets...),
		Configurator:    manifest.Configurator,
		SmokeFlow:       manifest.SmokeFlow,
		DashboardPolicy: manifest.Dashboard,
		SSOMode:         manifest.SSO.Mode,
		Health:          manifest.Health,
		Docs:            append([]string(nil), manifest.Docs...),
	}
}
