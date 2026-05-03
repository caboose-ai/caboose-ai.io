# Social Login (GitHub & Google) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a credential resolution chain (config → 1Password → TUI prompt → skip) for GitHub and Google OAuth, feeding into the existing `social.Configurator` that creates Authentik federation sources.

**Architecture:** The `SecretDef` struct gets an `Optional` flag. A new `ResolveSocialCredentials` method on `Installer` walks the resolution chain (YAML config → 1Password → prompt callback) and populates `Config.Social`. The TUI wires this between secrets generation and service configuration.

**Tech Stack:** Go, Bubbletea TUI, 1Password CLI (`op`), Authentik API

---

### File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/secrets/store.go` | Modify | Add `Optional` field to `SecretDef` |
| `internal/install/social.go` | Create | `ResolveSocialCredentials` method |
| `internal/install/social_test.go` | Create | Tests for credential resolution |
| `internal/tui/app.go` | Modify | Wire social credential phase into TUI flow |

---

### Task 1: Add `Optional` field to `SecretDef`

**Files:**
- Modify: `internal/secrets/store.go`

- [ ] **Step 1: Add the `Optional` field**

In `internal/secrets/store.go`, add `Optional bool` to the `SecretDef` struct:

```go
type SecretDef struct {
	Key      string
	Length   int
	Opts     GenerateOpts
	Prompt   bool
	Optional bool
}
```

- [ ] **Step 2: Add `SocialSecrets()` function**

Below `BootstrapSecrets()` in the same file, add:

```go
func SocialSecrets() []SecretDef {
	return []SecretDef{
		{Key: "GITHUB_OAUTH_CLIENT_ID", Prompt: true, Optional: true},
		{Key: "GITHUB_OAUTH_CLIENT_SECRET", Prompt: true, Optional: true},
		{Key: "GOOGLE_OAUTH_CLIENT_ID", Prompt: true, Optional: true},
		{Key: "GOOGLE_OAUTH_CLIENT_SECRET", Prompt: true, Optional: true},
	}
}
```

- [ ] **Step 3: Verify build**

Run: `go build ./internal/secrets/...`
Expected: clean build, no errors

- [ ] **Step 4: Commit**

```bash
git add internal/secrets/store.go
git commit -m "feat(secrets): add Optional field to SecretDef and SocialSecrets list"
```

---

### Task 2: Implement `ResolveSocialCredentials`

**Files:**
- Create: `internal/install/social.go`
- Test: `internal/install/social_test.go`

- [ ] **Step 1: Write the test**

Create `internal/install/social_test.go`:

```go
package install

import (
	"context"
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
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
	// Verify stored to secret store
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
		return "", nil // blank = skip
	}

	err := inst.ResolveSocialCredentials(context.Background(), promptFn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inst.Config.Social.GitHub != nil {
		t.Errorf("GitHub should be nil when skipped, got %+v", inst.Config.Social.GitHub)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/install/ -run TestResolveSocial -v`
Expected: compilation error — `ResolveSocialCredentials` not defined

- [ ] **Step 3: Write the implementation**

Create `internal/install/social.go`:

```go
package install

import (
	"context"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
)

type socialProvider struct {
	name        string
	idKey       string
	secretKey   string
	getCreds    func(*config.SocialConfig) *config.OAuthCredentials
	setCreds    func(*config.SocialConfig, *config.OAuthCredentials)
}

var socialProviders = []socialProvider{
	{
		name:      "GitHub",
		idKey:     "GITHUB_OAUTH_CLIENT_ID",
		secretKey: "GITHUB_OAUTH_CLIENT_SECRET",
		getCreds:  func(s *config.SocialConfig) *config.OAuthCredentials { return s.GitHub },
		setCreds:  func(s *config.SocialConfig, c *config.OAuthCredentials) { s.GitHub = c },
	},
	{
		name:      "Google",
		idKey:     "GOOGLE_OAUTH_CLIENT_ID",
		secretKey: "GOOGLE_OAUTH_CLIENT_SECRET",
		getCreds:  func(s *config.SocialConfig) *config.OAuthCredentials { return s.Google },
		setCreds:  func(s *config.SocialConfig, c *config.OAuthCredentials) { s.Google = c },
	},
}

func (inst *Installer) ResolveSocialCredentials(ctx context.Context, promptFn func(key string) (string, error)) error {
	for _, sp := range socialProviders {
		creds := sp.getCreds(&inst.Config.Social)
		if creds != nil && creds.ClientID != "" && creds.ClientSecret != "" {
			continue
		}

		clientID, _ := inst.Secrets.Get(ctx, sp.idKey)
		clientSecret, _ := inst.Secrets.Get(ctx, sp.secretKey)

		if clientID == "" && promptFn != nil {
			var err error
			clientID, err = promptFn(sp.idKey)
			if err != nil {
				return err
			}
		}
		if clientID == "" {
			continue
		}

		if clientSecret == "" && promptFn != nil {
			var err error
			clientSecret, err = promptFn(sp.secretKey)
			if err != nil {
				return err
			}
		}
		if clientSecret == "" {
			continue
		}

		sp.setCreds(&inst.Config.Social, &config.OAuthCredentials{
			ClientID:     clientID,
			ClientSecret: clientSecret,
		})

		_ = inst.Secrets.Put(ctx, sp.idKey, clientID)
		_ = inst.Secrets.Put(ctx, sp.secretKey, clientSecret)
	}
	return nil
}
```

- [ ] **Step 4: Fix test import**

The test file needs the `secrets` import for `mockSecretStore`. Update the import and the `Generate` method signature:

```go
import (
	"context"
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
	"github.com/caboose-ai/caboose-ai.io/internal/secrets"
)

func (m *mockSecretStore) Generate(_ context.Context, _ string, _ int, _ secrets.GenerateOpts) (string, error) {
	return "", nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/install/ -run TestResolveSocial -v`
Expected: all 4 tests PASS

- [ ] **Step 6: Commit**

```bash
git add internal/install/social.go internal/install/social_test.go
git commit -m "feat(install): add ResolveSocialCredentials with config/1Password/prompt chain"
```

---

### Task 3: Wire social credentials into the TUI flow

**Files:**
- Modify: `internal/tui/app.go`

The social credential phase runs between `SecretsCompleteMsg` and `ComposeUp`. The TUI needs to:
1. Display callback URLs for reference
2. Prompt for each missing credential (or skip)
3. Then continue to compose phase

- [ ] **Step 1: Add the `socialReadyMsg` type**

At the top of `internal/tui/app.go`, add a new message type alongside the existing ones:

```go
type socialReadyMsg struct{}
```

- [ ] **Step 2: Change `SecretsCompleteMsg` handler to start social phase**

Replace the `SecretsCompleteMsg` case:

```go
	case views.SecretsCompleteMsg:
		return m, m.runResolveSocialCredentials()
```

- [ ] **Step 3: Add `socialReadyMsg` handler to continue to compose**

Add a new case after `SecretsCompleteMsg`:

```go
	case socialReadyMsg:
		m.stepper.Current = 1
		return m, m.runComposeAndHealth()
```

- [ ] **Step 4: Add `runResolveSocialCredentials` method**

Add this method to `app.go`:

```go
func (m AppModel) runResolveSocialCredentials() tea.Cmd {
	promptVals := make(map[string]string, len(m.promptValues))
	for k, v := range m.promptValues {
		promptVals[k] = v
	}
	return func() tea.Msg {
		ctx := context.Background()
		err := m.installer.ResolveSocialCredentials(ctx, func(key string) (string, error) {
			if val, ok := promptVals[key]; ok && val != "" {
				return val, nil
			}
			return "", nil // skip if not provided
		})
		if err != nil {
			return views.SecretsErrorMsg{Err: fmt.Errorf("resolving social credentials: %w", err)}
		}
		return socialReadyMsg{}
	}
}
```

- [ ] **Step 5: Verify build**

Run: `go build ./internal/tui/...`
Expected: clean build

- [ ] **Step 6: Commit**

```bash
git add internal/tui/app.go
git commit -m "feat(tui): wire social credential resolution between secrets and compose phases"
```

---

### Task 4: Integration test and final verification

**Files:**
- Test: `internal/install/social_test.go` (add integration scenario)

- [ ] **Step 1: Add a test for the full resolution priority**

Append to `internal/install/social_test.go`:

```go
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

	promptCalled := false
	err := inst.ResolveSocialCredentials(context.Background(), func(key string) (string, error) {
		promptCalled = true
		return "prompt-val", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inst.Config.Social.GitHub.ClientID != "cfg-id" {
		t.Errorf("config should take priority, got %q", inst.Config.Social.GitHub.ClientID)
	}
	if promptCalled {
		t.Error("prompt should not be called when config has values")
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
```

- [ ] **Step 2: Run all tests**

Run: `go test ./internal/... -v`
Expected: all tests PASS

- [ ] **Step 3: Run full build**

Run: `go build ./...`
Expected: clean build

- [ ] **Step 4: Commit**

```bash
git add internal/install/social_test.go
git commit -m "test(install): add priority and multi-provider tests for social credential resolution"
```

---

### Task 5: Final commit and cleanup

- [ ] **Step 1: Verify all tests pass**

Run: `go test ./internal/... -v -count=1`
Expected: all tests PASS

- [ ] **Step 2: Verify clean build**

Run: `go build ./...`
Expected: clean build, no warnings

- [ ] **Step 3: Review the diff**

Run: `git diff main --stat`
Expected: changes only in the files listed in the file map, plus the test file
