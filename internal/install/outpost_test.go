package install

import (
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
)

func TestDefaultProxySpecsIncludePaperclip(t *testing.T) {
	urls := config.DeriveURLs("example.com")

	want := map[string]struct {
		slug string
		host string
	}{
		"ghost-proxy":     {slug: "ghost-proxy", host: "https://blog.example.com"},
		"paperclip-proxy": {slug: "paperclip-proxy", host: "https://paperclip.example.com"},
	}
	found := map[string]bool{}
	for _, spec := range DefaultProxySpecs(urls) {
		if expected, ok := want[spec.Name]; ok {
			found[spec.Name] = true
			if spec.Slug != expected.slug {
				t.Fatalf("%s Slug = %q, want %q", spec.Name, spec.Slug, expected.slug)
			}
			if spec.ExternalHost != expected.host {
				t.Fatalf("%s ExternalHost = %q, want %q", spec.Name, spec.ExternalHost, expected.host)
			}
		}
	}
	for name := range want {
		if !found[name] {
			t.Fatalf("%s missing from proxy specs", name)
		}
	}
}
