package smoketest

import (
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
)

func TestOAuthServiceFlowsIncludeNativeSSOApps(t *testing.T) {
	urls := config.DeriveURLs("example.com")
	flows := OAuthServiceFlows(urls)

	for _, name := range []string{"forgejo", "grafana", "portainer", "mattermost", "open-webui", "homarr"} {
		t.Run(name, func(t *testing.T) {
			if !hasServiceFlow(flows, name) {
				t.Fatalf("OAuthServiceFlows missing %q", name)
			}
		})
	}
}

func TestProxyFlowsIncludeOpenClaw(t *testing.T) {
	urls := config.DeriveURLs("example.com")
	flows := ProxyFlows(urls)

	for _, name := range []string{"dashboard", "dashboard-alias", "ci", "n8n", "openclaw"} {
		t.Run(name, func(t *testing.T) {
			if !hasProxyFlow(flows, name) {
				t.Fatalf("ProxyFlows missing %q", name)
			}
		})
	}
}

func hasServiceFlow(flows []ServiceFlow, name string) bool {
	for _, flow := range flows {
		if flow.Name == name {
			return true
		}
	}
	return false
}

func hasProxyFlow(flows []ProxyFlow, name string) bool {
	for _, flow := range flows {
		if flow.Name == name {
			return true
		}
	}
	return false
}
