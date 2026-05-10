package smoketest

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
	"github.com/caboose-ai/caboose-ai.io/internal/service"
)

func TestPaperclipSmokeFlowResolvesToProxyFlow(t *testing.T) {
	urls := config.DeriveURLs("example.com")
	flow, ok := SmokeFlowByName(urls, "paperclip")
	if !ok {
		t.Fatal("paperclip smoke flow did not resolve")
	}
	if flow.Proxy == nil {
		t.Fatalf("paperclip flow resolved to %+v, want proxy flow", flow)
	}
	if flow.Proxy.URL != "https://paperclip.example.com" {
		t.Fatalf("paperclip proxy URL = %q", flow.Proxy.URL)
	}
	if flow.Proxy.HealthURL != "http://127.0.0.1:3100/api/health" {
		t.Fatalf("paperclip health URL = %q", flow.Proxy.HealthURL)
	}
	if len(flow.Proxy.HealthBodyContains) == 0 {
		t.Fatal("paperclip smoke flow should assert local health response content")
	}
}

func TestServiceManifestSmokeFlowsResolve(t *testing.T) {
	manifests := loadSmokeTestManifests(t)
	urls := config.DeriveURLs("example.com")

	for slug, manifest := range manifests {
		if manifest.SmokeFlow == "" {
			continue
		}
		t.Run(slug, func(t *testing.T) {
			flow, ok := SmokeFlowByName(urls, manifest.SmokeFlow)
			if !ok {
				t.Fatalf("smoke_flow %q did not resolve to an executable smoke flow", manifest.SmokeFlow)
			}
			if flow.Name != manifest.SmokeFlow {
				t.Fatalf("resolved flow name = %q, want %q", flow.Name, manifest.SmokeFlow)
			}
		})
	}
}

func TestHealthOnlyServicesDoNotDeclareSmokeFlows(t *testing.T) {
	manifests := loadSmokeTestManifests(t)

	for _, slug := range []string{"cadvisor", "loki", "prometheus", "promtail", "sonarqube"} {
		t.Run(slug, func(t *testing.T) {
			if manifests[slug].SmokeFlow != "" {
				t.Fatalf("%s smoke_flow = %q, want empty", slug, manifests[slug].SmokeFlow)
			}
		})
	}
}

func loadSmokeTestManifests(t *testing.T) map[string]service.Manifest {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "services"))
	manifests, err := service.LoadManifests(root)
	if err != nil {
		t.Fatalf("LoadManifests: %v", err)
	}
	return manifests
}
