package install

import (
	"context"
	"fmt"
	"strings"

	"github.com/caboose-ai/caboose-ai.io/internal/services/authentik"
)

type providerFlows struct {
	Authorization *authentik.Flow
	Invalidation  *authentik.Flow
}

func (inst *Installer) ensureProviderFlows(ctx context.Context) (*providerFlows, error) {
	authFlow, err := inst.ensureFlow(ctx, authentik.CreateFlowParams{
		Name:        "Provider authorization implicit consent",
		Slug:        "default-provider-authorization-implicit-consent",
		Title:       "Redirecting to application",
		Designation: "authorization",
	})
	if err != nil {
		return nil, fmt.Errorf("ensuring authorization flow: %w", err)
	}

	invalidationFlow, err := inst.ensureFlow(ctx, authentik.CreateFlowParams{
		Name:        "Provider invalidation flow",
		Slug:        "default-provider-invalidation-flow",
		Title:       "You've logged out of the application.",
		Designation: "invalidation",
	})
	if err != nil {
		return nil, fmt.Errorf("ensuring invalidation flow: %w", err)
	}

	return &providerFlows{Authorization: authFlow, Invalidation: invalidationFlow}, nil
}

func (inst *Installer) ensureFlow(ctx context.Context, params authentik.CreateFlowParams) (*authentik.Flow, error) {
	flow, err := inst.AK.GetFlow(ctx, params.Slug)
	if err == nil {
		return flow, nil
	}
	if !strings.Contains(err.Error(), "not found") {
		return nil, err
	}

	flow, err = inst.AK.CreateFlow(ctx, params)
	if err == nil {
		return flow, nil
	}
	if strings.Contains(err.Error(), "already exists") {
		return inst.AK.GetFlow(ctx, params.Slug)
	}
	return nil, err
}
