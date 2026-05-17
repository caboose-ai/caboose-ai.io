package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/mcpaccess"
)

func TestRunMCPAccessClientRequestPrintsBlobAndStoresPendingKey(t *testing.T) {
	pendingDir := t.TempDir()
	var stdout, stderr bytes.Buffer

	code := RunMCPAccessClient(context.Background(), &stdout, &stderr, []string{
		"request",
		"--name", "codex on caboose",
		"--endpoint", "https://mcp.example.com",
		"--pending-dir", pendingDir,
		"--out", "-",
	}, MCPAccessClientOptions{})
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	var req mcpaccess.AccessRequest
	if err := json.Unmarshal(stdout.Bytes(), &req); err != nil {
		t.Fatalf("stdout was not a request JSON blob: %v\n%s", err, stdout.String())
	}
	if req.Name != "codex on caboose" || req.Endpoint != "https://mcp.example.com" || req.Scope != mcpaccess.ScopeHomelab {
		t.Fatalf("request = %#v", req)
	}
	if _, err := os.Stat(filepath.Join(pendingDir, req.ID+".key")); err != nil {
		t.Fatalf("pending key not written: %v", err)
	}
	if strings.Contains(stderr.String(), req.PublicKey) {
		t.Fatalf("stderr should not echo request blob: %q", stderr.String())
	}
}

func TestRunMCPAccessClientStatusMissingCredentialExplainsStartupFix(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var stdout, stderr bytes.Buffer

	code := RunMCPAccessClient(context.Background(), &stdout, &stderr, []string{"status"}, MCPAccessClientOptions{})
	if code == 0 {
		t.Fatalf("exit code = 0, want failure")
	}
	message := stderr.String()
	for _, want := range []string{
		"No Homelab MCP access credential found",
		"homelab mcp access setup",
		"homelab-mcp access request",
		"homelab-mcp access import",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("stderr missing %q:\n%s", want, message)
		}
	}
}
