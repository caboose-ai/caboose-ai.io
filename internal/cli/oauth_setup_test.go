package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
)

func TestPrintOAuthSetup(t *testing.T) {
	var out bytes.Buffer
	PrintOAuthSetup(&out, &config.Config{Domain: "example.com"})
	text := out.String()

	want := []string{
		"https://auth.example.com/source/oauth/callback/github/",
		"https://auth.example.com/source/oauth/callback/google/",
		"Hostname: auth.example.com",
		"GITHUB_OAUTH_CLIENT_ID",
		"GOOGLE_OAUTH_CLIENT_SECRET",
		"TURNSTILE_SECRET_KEY",
	}
	for _, s := range want {
		if !strings.Contains(text, s) {
			t.Fatalf("setup output missing %q:\n%s", s, text)
		}
	}
}
