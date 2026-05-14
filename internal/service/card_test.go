package service

import (
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
)

func TestBuildCardsDerivesAgentContextFromManifests(t *testing.T) {
	show := true
	registry := NewRegistry(map[string]Manifest{
		"paperclip": {
			Slug:            "paperclip",
			DisplayName:     "Paperclip",
			URLKey:          "paperclip",
			ComposeServices: []string{"paperclip", "paperclip-db"},
			Secrets:         []string{"PAPERCLIP_DB_PASS", "PAPERCLIP_AUTH_SECRET"},
			Configurator:    "paperclip",
			SmokeFlow:       "paperclip",
			Dashboard:       DashboardSpec{Show: &show},
			SSO:             SSOSpec{Mode: "proxy"},
			Health:          HealthSpec{URLKey: "paperclip", Path: "/api/health", Kind: "http"},
			Docs:            []string{"README.md", "context.md"},
		},
	}, nil)

	cards := BuildCards(registry, config.DeriveURLs("example.com"))

	if len(cards) != 1 {
		t.Fatalf("BuildCards returned %d cards, want 1", len(cards))
	}
	card := cards[0]
	if card.Slug != "paperclip" || card.DisplayName != "Paperclip" {
		t.Fatalf("card identity = %q/%q, want paperclip/Paperclip", card.Slug, card.DisplayName)
	}
	if card.Runtime != RuntimeCompose {
		t.Fatalf("card runtime = %q, want %q", card.Runtime, RuntimeCompose)
	}
	if card.URLKey != "paperclip" || card.ResolvedURL != "https://paperclip.example.com" {
		t.Fatalf("card URL = key %q resolved %q", card.URLKey, card.ResolvedURL)
	}
	if got := card.ComposeServices; len(got) != 2 || got[0] != "paperclip" || got[1] != "paperclip-db" {
		t.Fatalf("compose services = %v, want [paperclip paperclip-db]", got)
	}
	if got := card.SecretNames; len(got) != 2 || got[0] != "PAPERCLIP_DB_PASS" || got[1] != "PAPERCLIP_AUTH_SECRET" {
		t.Fatalf("secret names = %v", got)
	}
	if card.Configurator != "paperclip" || card.SmokeFlow != "paperclip" {
		t.Fatalf("configurator/smoke_flow = %q/%q", card.Configurator, card.SmokeFlow)
	}
	if card.DashboardPolicy.Show == nil || !*card.DashboardPolicy.Show {
		t.Fatalf("dashboard policy = %+v, want show true", card.DashboardPolicy)
	}
	if card.SSOMode != "proxy" {
		t.Fatalf("sso mode = %q, want proxy", card.SSOMode)
	}
	if card.Health.Path != "/api/health" || card.Health.Kind != "http" || card.Health.URLKey != "paperclip" {
		t.Fatalf("health = %+v", card.Health)
	}
	if got := card.Docs; len(got) != 2 || got[0] != "README.md" || got[1] != "context.md" {
		t.Fatalf("docs = %v", got)
	}
}

func TestBuildCardsSortsBySlugAndKeepsUnknownURLKeyUnresolved(t *testing.T) {
	registry := NewRegistry(map[string]Manifest{
		"zeta": {Slug: "zeta", DisplayName: "Zeta", URLKey: "missing"},
		"alpha": {
			Slug:        "alpha",
			DisplayName: "Alpha",
			Runtime:     RuntimeExternal,
			URLKey:      "",
		},
	}, nil)

	cards := BuildCards(registry, config.DeriveURLs("example.com"))

	if got := []string{cards[0].Slug, cards[1].Slug}; got[0] != "alpha" || got[1] != "zeta" {
		t.Fatalf("card order = %v, want [alpha zeta]", got)
	}
	if cards[0].Runtime != RuntimeExternal {
		t.Fatalf("alpha runtime = %q, want external", cards[0].Runtime)
	}
	if cards[0].ResolvedURL != "" {
		t.Fatalf("alpha resolved URL = %q, want empty", cards[0].ResolvedURL)
	}
	if cards[1].URLKey != "missing" || cards[1].ResolvedURL != "" {
		t.Fatalf("zeta URL key/resolved = %q/%q, want missing/empty", cards[1].URLKey, cards[1].ResolvedURL)
	}
}
