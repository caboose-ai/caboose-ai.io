package secrets

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/runner"
)

func TestOnePasswordStoreUsesStaticVaultForExternalSecrets(t *testing.T) {
	ctx := context.Background()
	r := runner.NewMockRunner()
	r.OnFunc("op item get GITHUB_OAUTH_CLIENT_ID", func(args []string) ([]byte, error) {
		if !slices.Contains(args, "Homelab - Static") {
			return nil, fmt.Errorf("expected static vault args, got %v", args)
		}
		return []byte(`{"value":"github-client-id"}`), nil
	})

	store := NewSplitOnePasswordStore("Homelab - Static", "Homelab - Dynamic", r, filepath.Join(t.TempDir(), ".env"))
	got, err := store.Get(ctx, "GITHUB_OAUTH_CLIENT_ID")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "github-client-id" {
		t.Fatalf("Get = %q, want github-client-id", got)
	}
}

func TestOnePasswordStoreWritesGeneratedSecretsToDynamicVault(t *testing.T) {
	ctx := context.Background()
	r := runner.NewMockRunner()
	r.On("op item get AUTHENTIK_BOOTSTRAP_TOKEN", nil, fmt.Errorf("missing"))
	r.OnFunc("op item create", func(args []string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if !strings.Contains(joined, "--vault=Homelab - Dynamic") {
			return nil, fmt.Errorf("expected dynamic vault args, got %v", args)
		}
		return []byte(`{"fields":[{"id":"password","value":"generated-token"}]}`), nil
	})

	store := NewSplitOnePasswordStore("Homelab - Static", "Homelab - Dynamic", r, filepath.Join(t.TempDir(), ".env"))
	got, err := store.Generate(ctx, "AUTHENTIK_BOOTSTRAP_TOKEN", 32, GenerateOpts{Recipe: "letters,digits,32"})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if got != "generated-token" {
		t.Fatalf("Generate = %q, want generated-token", got)
	}
}

func TestOnePasswordStoreGenerateUsesExistingSecretWhenCreateFindsDuplicate(t *testing.T) {
	ctx := context.Background()
	r := runner.NewMockRunner()
	r.On("op item get AUTHENTIK_SECRET_KEY", []byte(`{"value":"existing-secret"}`), nil)
	r.On("op item create", nil, fmt.Errorf("item already exists"))

	store := NewSplitOnePasswordStore("Homelab - Static", "Homelab - Dynamic", r, filepath.Join(t.TempDir(), ".env"))
	got, err := store.Generate(ctx, "AUTHENTIK_SECRET_KEY", 50, GenerateOpts{Recipe: "letters,digits,symbols,50"})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if got != "existing-secret" {
		t.Fatalf("Generate = %q, want existing-secret", got)
	}
}

func TestDerivedSecretKeysExcludesTurnstileCredentials(t *testing.T) {
	for _, key := range DerivedSecretKeys() {
		if key == "TURNSTILE_SITE_KEY" || key == "TURNSTILE_SECRET_KEY" {
			t.Fatalf("DerivedSecretKeys contains static external credential %s", key)
		}
	}
}

func TestOnePasswordStoreDeleteMissingItemStillDeletesEnvValue(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("AUTHENTIK_SECRET_KEY=old-value\n"), 0600); err != nil {
		t.Fatalf("write env: %v", err)
	}

	r := runner.NewMockRunner()
	r.On("op item get AUTHENTIK_SECRET_KEY", nil, fmt.Errorf("missing"))

	store := NewSplitOnePasswordStore("Homelab - Static", "Homelab - Dynamic", r, envPath)
	if err := store.Delete(ctx, "AUTHENTIK_SECRET_KEY"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	data, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("read env: %v", err)
	}
	if strings.Contains(string(data), "AUTHENTIK_SECRET_KEY=") {
		t.Fatalf("Delete left env value behind:\n%s", data)
	}
	if len(r.Calls) != 1 {
		t.Fatalf("Delete should not call op item delete for missing item, calls=%v", r.Calls)
	}
}

func TestOnePasswordStoreDeleteUsesItemID(t *testing.T) {
	ctx := context.Background()
	r := runner.NewMockRunner()
	r.On("op item get AUTHENTIK_SECRET_KEY", []byte(`{"id":"item-uuid"}`), nil)
	r.OnFunc("op item delete", func(args []string) ([]byte, error) {
		if len(args) < 3 || args[2] != "item-uuid" {
			return nil, fmt.Errorf("expected delete by item ID, got args %v", args)
		}
		return nil, nil
	})

	store := NewSplitOnePasswordStore("Homelab - Static", "Homelab - Dynamic", r, filepath.Join(t.TempDir(), ".env"))
	if err := store.Delete(ctx, "AUTHENTIK_SECRET_KEY"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}
