package install

import (
	"context"
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
)

func TestResolveTurnstileCredentials(t *testing.T) {
	tests := []struct {
		name            string
		cfgSiteKey      string
		cfgSecretKey    string
		secretsData     map[string]string
		promptResponses map[string]string
		wantSiteKey     string
		wantSecretKey   string
		wantInSecrets   map[string]string
		promptCalled    bool
	}{
		{
			name:         "config provides both — stored to secrets, no prompt called",
			cfgSiteKey:   "cfg-site",
			cfgSecretKey: "cfg-secret",
			secretsData:  map[string]string{},
			wantSiteKey:  "cfg-site",
			wantSecretKey: "cfg-secret",
			wantInSecrets: map[string]string{
				"TURNSTILE_SITE_KEY":   "cfg-site",
				"TURNSTILE_SECRET_KEY": "cfg-secret",
			},
			promptCalled: false,
		},
		{
			name:        "secrets store provides both — stored to config",
			secretsData: map[string]string{
				"TURNSTILE_SITE_KEY":   "op-site",
				"TURNSTILE_SECRET_KEY": "op-secret",
			},
			wantSiteKey:   "op-site",
			wantSecretKey: "op-secret",
			wantInSecrets: map[string]string{
				"TURNSTILE_SITE_KEY":   "op-site",
				"TURNSTILE_SECRET_KEY": "op-secret",
			},
			promptCalled: false,
		},
		{
			name:        "promptFn provides both — stored to config and secrets",
			secretsData: map[string]string{},
			promptResponses: map[string]string{
				"TURNSTILE_SITE_KEY":   "prompt-site",
				"TURNSTILE_SECRET_KEY": "prompt-secret",
			},
			wantSiteKey:   "prompt-site",
			wantSecretKey: "prompt-secret",
			wantInSecrets: map[string]string{
				"TURNSTILE_SITE_KEY":   "prompt-site",
				"TURNSTILE_SECRET_KEY": "prompt-secret",
			},
			promptCalled: true,
		},
		{
			name:        "site key provided but secret key empty — skip",
			secretsData: map[string]string{
				"TURNSTILE_SITE_KEY": "op-site",
			},
			promptResponses: map[string]string{
				"TURNSTILE_SECRET_KEY": "",
			},
			wantSiteKey:   "",
			wantSecretKey: "",
			wantInSecrets: map[string]string{},
			promptCalled:  true,
		},
		{
			name:          "both missing — skip",
			secretsData:   map[string]string{},
			wantSiteKey:   "",
			wantSecretKey: "",
			wantInSecrets: map[string]string{},
			promptCalled:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.Domain = "test.example.com"
			cfg.Turnstile.SiteKey = tt.cfgSiteKey
			cfg.Turnstile.SecretKey = tt.cfgSecretKey

			store := &mockSecretStore{data: make(map[string]string)}
			for k, v := range tt.secretsData {
				store.data[k] = v
			}

			inst := &Installer{
				Config:  cfg,
				State:   NewState(),
				Secrets: store,
			}

			prompted := false
			var promptFn func(string) (string, error)
			if tt.promptResponses != nil {
				promptFn = func(key string) (string, error) {
					prompted = true
					return tt.promptResponses[key], nil
				}
			}

			err := inst.ResolveTurnstileCredentials(context.Background(), promptFn)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if inst.Config.Turnstile.SiteKey != tt.wantSiteKey {
				t.Errorf("Config.Turnstile.SiteKey = %q, want %q", inst.Config.Turnstile.SiteKey, tt.wantSiteKey)
			}
			if inst.Config.Turnstile.SecretKey != tt.wantSecretKey {
				t.Errorf("Config.Turnstile.SecretKey = %q, want %q", inst.Config.Turnstile.SecretKey, tt.wantSecretKey)
			}

			for k, want := range tt.wantInSecrets {
				got := store.data[k]
				if got != want {
					t.Errorf("secrets[%q] = %q, want %q", k, got, want)
				}
			}

			if tt.promptCalled && !prompted {
				t.Error("expected promptFn to be called, but it was not")
			}
			if !tt.promptCalled && prompted {
				t.Error("expected promptFn not to be called, but it was")
			}
		})
	}
}
