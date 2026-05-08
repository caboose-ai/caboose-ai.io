package install

import (
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
)

func TestDefaultProxySpecsIncludePaperclip(t *testing.T) {
	urls := config.DeriveURLs("example.com")

	var found bool
	for _, spec := range DefaultProxySpecs(urls) {
		if spec.Name == "paperclip-proxy" {
			found = true
			if spec.Slug != "paperclip-proxy" {
				t.Fatalf("Slug = %q, want paperclip-proxy", spec.Slug)
			}
			if spec.ExternalHost != "https://paperclip.example.com" {
				t.Fatalf("ExternalHost = %q", spec.ExternalHost)
			}
		}
	}
	if !found {
		t.Fatal("paperclip-proxy missing from proxy specs")
	}
}
