package install

import (
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
)

func TestDefaultProxySpecsIncludePaperclip(t *testing.T) {
	urls := config.DeriveURLs("example.com")

	want := map[string]struct {
		slug         string
		externalHost string
		internalHost string
	}{
		"ci-proxy":        {slug: "ci-proxy", externalHost: "https://ci.example.com", internalHost: "http://woodpecker-server:8000"},
		"ghost-proxy":     {slug: "ghost-proxy", externalHost: "https://blog.example.com", internalHost: "http://ghost:2368"},
		"paperclip-proxy": {slug: "paperclip-proxy", externalHost: "https://paperclip.example.com"},
	}
	found := map[string]bool{}
	for _, spec := range DefaultProxySpecs(urls) {
		if expected, ok := want[spec.Name]; ok {
			found[spec.Name] = true
			if spec.Slug != expected.slug {
				t.Fatalf("%s Slug = %q, want %q", spec.Name, spec.Slug, expected.slug)
			}
			if spec.ExternalHost != expected.externalHost {
				t.Fatalf("%s ExternalHost = %q, want %q", spec.Name, spec.ExternalHost, expected.externalHost)
			}
			if spec.InternalHost != expected.internalHost {
				t.Fatalf("%s InternalHost = %q, want %q", spec.Name, spec.InternalHost, expected.internalHost)
			}
		}
	}
	for name := range want {
		if !found[name] {
			t.Fatalf("%s missing from proxy specs", name)
		}
	}
}
