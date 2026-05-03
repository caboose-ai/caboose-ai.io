package install

import (
	"context"
	"fmt"

	"github.com/caboose-ai/caboose-ai.io/internal/services/authentik"
)

type ProxySpec struct {
	Name         string
	Slug         string
	ExternalHost string
}

func (inst *Installer) ProxySpecs() []ProxySpec {
	urls := inst.Config.URLs()
	return []ProxySpec{
		{Name: "dashboard", Slug: "dashboard", ExternalHost: urls.Dashboard},
		{Name: "dashboard-alias", Slug: "dashboard-alias", ExternalHost: urls.DashAlias},
		{Name: "ci-proxy", Slug: "ci-proxy", ExternalHost: urls.CI},
		{Name: "n8n-proxy", Slug: "n8n-proxy", ExternalHost: urls.N8N},
		{Name: "openclaw-proxy", Slug: "openclaw-proxy", ExternalHost: urls.OpenClaw},
	}
}

type OutpostProgress struct {
	Name   string
	Action string // "exists", "creating", "created", "binding", "bound", "error"
	Err    error
}

func (inst *Installer) ProvisionOutpost(ctx context.Context, progressFn func(OutpostProgress)) error {
	if progressFn == nil {
		progressFn = func(OutpostProgress) {}
	}

	if inst.State.DryRun {
		for _, spec := range inst.ProxySpecs() {
			progressFn(OutpostProgress{Name: spec.Name, Action: "created"})
		}
		progressFn(OutpostProgress{Name: "outpost", Action: "bound"})
		return nil
	}

	authFlow, err := inst.AK.GetFlow(ctx, "default-provider-authorization-implicit-consent")
	if err != nil {
		return fmt.Errorf("looking up authorization flow: %w", err)
	}

	invalidationFlow, err := inst.AK.GetFlow(ctx, "default-provider-invalidation-flow")
	if err != nil {
		return fmt.Errorf("looking up invalidation flow: %w", err)
	}

	var providerPKs []int

	for _, spec := range inst.ProxySpecs() {
		existing, _ := inst.AK.GetProxyProvider(ctx, spec.Name)
		if existing != nil {
			progressFn(OutpostProgress{Name: spec.Name, Action: "exists"})
			providerPKs = append(providerPKs, existing.PK)
			continue
		}

		progressFn(OutpostProgress{Name: spec.Name, Action: "creating"})

		provider, err := inst.AK.CreateProxyProvider(ctx, authentik.CreateProxyProviderParams{
			Name:              spec.Name,
			AuthorizationFlow: authFlow.PK,
			InvalidationFlow:  invalidationFlow.PK,
			Mode:              "forward_single",
			ExternalHost:      spec.ExternalHost,
		})
		if err != nil {
			progressFn(OutpostProgress{Name: spec.Name, Action: "error", Err: err})
			return fmt.Errorf("creating proxy provider %q: %w", spec.Name, err)
		}

		app, _ := inst.AK.GetApplication(ctx, spec.Slug)
		if app == nil {
			if _, err := inst.AK.CreateApplication(ctx, spec.Name, spec.Slug, provider.PK); err != nil {
				progressFn(OutpostProgress{Name: spec.Name, Action: "error", Err: err})
				return fmt.Errorf("creating application %q: %w", spec.Slug, err)
			}
		}

		providerPKs = append(providerPKs, provider.PK)
		progressFn(OutpostProgress{Name: spec.Name, Action: "created"})
	}

	progressFn(OutpostProgress{Name: "outpost", Action: "binding"})

	outpost, err := inst.AK.GetEmbeddedOutpost(ctx)
	if err != nil {
		progressFn(OutpostProgress{Name: "outpost", Action: "error", Err: err})
		return fmt.Errorf("finding embedded outpost: %w", err)
	}

	urls := inst.Config.URLs()
	merged := mergeProviderPKs(outpost.Providers, providerPKs)
	if err := inst.AK.UpdateOutpost(ctx, outpost.PK, merged, urls.Authentik); err != nil {
		progressFn(OutpostProgress{Name: "outpost", Action: "error", Err: err})
		return fmt.Errorf("binding providers to outpost: %w", err)
	}

	progressFn(OutpostProgress{Name: "outpost", Action: "bound"})
	return nil
}

func mergeProviderPKs(existing, new []int) []int {
	seen := make(map[int]bool, len(existing))
	for _, pk := range existing {
		seen[pk] = true
	}
	merged := append([]int{}, existing...)
	for _, pk := range new {
		if !seen[pk] {
			merged = append(merged, pk)
			seen[pk] = true
		}
	}
	return merged
}
