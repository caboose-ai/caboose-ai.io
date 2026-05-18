package smoketest

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveAuthentikToken_FromEnv(t *testing.T) {
	t.Setenv("AUTHENTIK_TOKEN", "env-token")
	t.Setenv("AUTHENTIK_BOOTSTRAP_TOKEN", "")

	token, err := resolveAuthentikTokenFromPath(context.Background(), "", func(context.Context) (string, error) {
		t.Fatal("recover should not be called when AUTHENTIK_TOKEN is set")
		return "", nil
	})
	if err != nil {
		t.Fatalf("resolveAuthentikToken returned unexpected error: %v", err)
	}
	if token != "env-token" {
		t.Fatalf("token = %q, want %q", token, "env-token")
	}
}

func TestResolveAuthentikToken_FromBootstrapEnv(t *testing.T) {
	t.Setenv("AUTHENTIK_TOKEN", "")
	t.Setenv("AUTHENTIK_BOOTSTRAP_TOKEN", "bootstrap-token")

	token, err := resolveAuthentikTokenFromPath(context.Background(), "", func(context.Context) (string, error) {
		t.Fatal("recover should not be called when AUTHENTIK_BOOTSTRAP_TOKEN is set")
		return "", nil
	})
	if err != nil {
		t.Fatalf("resolveAuthentikToken returned unexpected error: %v", err)
	}
	if token != "bootstrap-token" {
		t.Fatalf("token = %q, want %q", token, "bootstrap-token")
	}
}

func TestResolveAuthentikToken_FromEnvFile(t *testing.T) {
	t.Setenv("AUTHENTIK_TOKEN", "")
	t.Setenv("AUTHENTIK_BOOTSTRAP_TOKEN", "")
	_ = os.Unsetenv("AUTHENTIK_TOKEN")
	_ = os.Unsetenv("AUTHENTIK_BOOTSTRAP_TOKEN")

	envPath := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(envPath, []byte("AUTHENTIK_TOKEN=file-token\nAUTHENTIK_BOOTSTRAP_TOKEN=bootstrap-file-token\n"), 0600); err != nil {
		t.Fatalf("writing env file: %v", err)
	}

	token, err := resolveAuthentikTokenFromPath(context.Background(), envPath, func(context.Context) (string, error) {
		t.Fatal("recover should not be called when .env token is set")
		return "", nil
	})
	if err != nil {
		t.Fatalf("resolveAuthentikToken returned unexpected error: %v", err)
	}
	if token != "file-token" {
		t.Fatalf("token = %q, want %q", token, "file-token")
	}
}

func TestResolveAuthentikToken_DoesNotRecoverByDefault(t *testing.T) {
	t.Setenv("AUTHENTIK_TOKEN", "")
	t.Setenv("AUTHENTIK_BOOTSTRAP_TOKEN", "")
	t.Setenv("SMOKETEST_RECOVER_AUTHENTIK_TOKEN", "")

	_ = os.Unsetenv("AUTHENTIK_TOKEN")
	_ = os.Unsetenv("AUTHENTIK_BOOTSTRAP_TOKEN")

	token, err := resolveAuthentikTokenFromPath(context.Background(), "", func(context.Context) (string, error) {
		t.Fatal("recover should not be called unless SMOKETEST_RECOVER_AUTHENTIK_TOKEN=1")
		return "", nil
	})
	if err != nil {
		t.Fatalf("resolveAuthentikToken returned unexpected error: %v", err)
	}
	if token != "" {
		t.Fatalf("token = %q, want empty token", token)
	}
}

func TestResolveAuthentikToken_RecoversWhenEnabled(t *testing.T) {
	t.Setenv("AUTHENTIK_TOKEN", "")
	t.Setenv("AUTHENTIK_BOOTSTRAP_TOKEN", "")
	t.Setenv("SMOKETEST_RECOVER_AUTHENTIK_TOKEN", "1")

	_ = os.Unsetenv("AUTHENTIK_TOKEN")
	_ = os.Unsetenv("AUTHENTIK_BOOTSTRAP_TOKEN")

	token, err := resolveAuthentikTokenFromPath(context.Background(), "", func(context.Context) (string, error) {
		return "recovered-token", nil
	})
	if err != nil {
		t.Fatalf("resolveAuthentikToken returned unexpected error: %v", err)
	}
	if token != "recovered-token" {
		t.Fatalf("token = %q, want %q", token, "recovered-token")
	}
}
