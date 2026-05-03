package install

import (
	"context"
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
	"github.com/caboose-ai/caboose-ai.io/internal/secrets"
)

type mockSecretStore struct {
	data map[string]string
}

func (m *mockSecretStore) Get(_ context.Context, key string) (string, error) {
	return m.data[key], nil
}
func (m *mockSecretStore) Put(_ context.Context, key, value string) error {
	m.data[key] = value
	return nil
}
func (m *mockSecretStore) Generate(_ context.Context, _ string, _ int, _ secrets.GenerateOpts) (string, error) {
	return "", nil
}
func (m *mockSecretStore) EnsureVault(_ context.Context) error { return nil }

func TestResolveSocialCredentials_FromConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Domain = "test.example.com"
	cfg.Social = config.SocialConfig{
		GitHub: &config.OAuthCredentials{ClientID: "cfg-gh-id", ClientSecret: "cfg-gh-secret"},
	}
	inst := &Installer{
		Config:  cfg,
		State:   NewState(),
		Secrets: &mockSecretStore{data: map[string]string{}},
	}

	err := inst.ResolveSocialCredentials(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inst.Config.Social.GitHub.ClientID != "cfg-gh-id" {
		t.Errorf("GitHub ClientID = %q, want cfg-gh-id", inst.Config.Social.GitHub.ClientID)
	}
}

func TestResolveSocialCredentials_From1Password(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Domain = "test.example.com"
	inst := &Installer{
		Config: cfg,
		State:  NewState(),
		Secrets: &mockSecretStore{data: map[string]string{
			"GITHUB_OAUTH_CLIENT_ID":     "op-gh-id",
			"GITHUB_OAUTH_CLIENT_SECRET": "op-gh-secret",
		}},
	}

	err := inst.ResolveSocialCredentials(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inst.Config.Social.GitHub == nil {
		t.Fatal("GitHub credentials not resolved from 1Password")
	}
	if inst.Config.Social.GitHub.ClientID != "op-gh-id" {
		t.Errorf("GitHub ClientID = %q, want op-gh-id", inst.Config.Social.GitHub.ClientID)
	}
}

func TestResolveSocialCredentials_FromPrompt(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Domain = "test.example.com"
	inst := &Installer{
		Config:  cfg,
		State:   NewState(),
		Secrets: &mockSecretStore{data: map[string]string{}},
	}

	promptResponses := map[string]string{
		"GITHUB_OAUTH_CLIENT_ID":     "prompt-gh-id",
		"GITHUB_OAUTH_CLIENT_SECRET": "prompt-gh-secret",
	}
	promptFn := func(key string) (string, error) {
		return promptResponses[key], nil
	}

	err := inst.ResolveSocialCredentials(context.Background(), promptFn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inst.Config.Social.GitHub == nil {
		t.Fatal("GitHub credentials not resolved from prompt")
	}
	if inst.Config.Social.GitHub.ClientID != "prompt-gh-id" {
		t.Errorf("GitHub ClientID = %q, want prompt-gh-id", inst.Config.Social.GitHub.ClientID)
	}
	val, _ := inst.Secrets.Get(context.Background(), "GITHUB_OAUTH_CLIENT_ID")
	if val != "prompt-gh-id" {
		t.Errorf("Secret not stored: got %q", val)
	}
}

func TestResolveSocialCredentials_SkipOnBlank(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Domain = "test.example.com"
	inst := &Installer{
		Config:  cfg,
		State:   NewState(),
		Secrets: &mockSecretStore{data: map[string]string{}},
	}

	promptFn := func(key string) (string, error) {
		return "", nil
	}

	err := inst.ResolveSocialCredentials(context.Background(), promptFn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inst.Config.Social.GitHub != nil {
		t.Errorf("GitHub should be nil when skipped, got %+v", inst.Config.Social.GitHub)
	}
}

func TestResolveSocialCredentials_ConfigTakesPriority(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Domain = "test.example.com"
	cfg.Social = config.SocialConfig{
		GitHub: &config.OAuthCredentials{ClientID: "cfg-id", ClientSecret: "cfg-secret"},
	}
	inst := &Installer{
		Config: cfg,
		State:  NewState(),
		Secrets: &mockSecretStore{data: map[string]string{
			"GITHUB_OAUTH_CLIENT_ID":     "op-id",
			"GITHUB_OAUTH_CLIENT_SECRET": "op-secret",
		}},
	}

	githubPromptCalled := false
	err := inst.ResolveSocialCredentials(context.Background(), func(key string) (string, error) {
		if key == "GITHUB_OAUTH_CLIENT_ID" || key == "GITHUB_OAUTH_CLIENT_SECRET" {
			githubPromptCalled = true
		}
		return "", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inst.Config.Social.GitHub.ClientID != "cfg-id" {
		t.Errorf("config should take priority, got %q", inst.Config.Social.GitHub.ClientID)
	}
	if githubPromptCalled {
		t.Error("prompt should not be called for GitHub when config has values")
	}
}

func TestResolveSocialCredentials_BothProviders(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Domain = "test.example.com"
	inst := &Installer{
		Config: cfg,
		State:  NewState(),
		Secrets: &mockSecretStore{data: map[string]string{
			"GITHUB_OAUTH_CLIENT_ID":     "gh-id",
			"GITHUB_OAUTH_CLIENT_SECRET": "gh-secret",
			"GOOGLE_OAUTH_CLIENT_ID":     "go-id",
			"GOOGLE_OAUTH_CLIENT_SECRET": "go-secret",
		}},
	}

	err := inst.ResolveSocialCredentials(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inst.Config.Social.GitHub == nil || inst.Config.Social.GitHub.ClientID != "gh-id" {
		t.Errorf("GitHub not resolved: %+v", inst.Config.Social.GitHub)
	}
	if inst.Config.Social.Google == nil || inst.Config.Social.Google.ClientID != "go-id" {
		t.Errorf("Google not resolved: %+v", inst.Config.Social.Google)
	}
}
