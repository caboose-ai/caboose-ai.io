package install

import (
	"context"
	"fmt"
	"strings"

	"github.com/caboose-ai/caboose-ai.io/services/authentik"
)

const (
	googleAccountLogoutRedirectStageName = "google-account-logout-redirect"
	googleAccountLogoutURL               = "https://accounts.google.com/Logout"
	defaultInvalidationFlowSlug          = "default-invalidation-flow"
	defaultUserLogoutStageName           = "default-invalidation-logout"
)

type providerFlows struct {
	Authorization *authentik.Flow
	Invalidation  *authentik.Flow
}

func (inst *Installer) ConfigureLogoutFlows(ctx context.Context) error {
	if inst.State.DryRun {
		return nil
	}

	providerFlows, err := inst.ensureProviderFlows(ctx)
	if err != nil {
		return fmt.Errorf("ensuring provider flows: %w", err)
	}

	globalInvalidationFlow, err := inst.ensureFlow(ctx, authentik.CreateFlowParams{
		Name:        "Logout",
		Slug:        defaultInvalidationFlowSlug,
		Title:       "Default Invalidation Flow",
		Designation: "invalidation",
	})
	if err != nil {
		return fmt.Errorf("ensuring default invalidation flow: %w", err)
	}

	return inst.ensureGoogleAccountLogoutRedirect(ctx, globalInvalidationFlow, providerFlows.Invalidation)
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

func (inst *Installer) ensureGoogleAccountLogoutRedirect(ctx context.Context, flows ...*authentik.Flow) error {
	logoutStage, err := inst.ensureUserLogoutStage(ctx, defaultUserLogoutStageName)
	if err != nil {
		return fmt.Errorf("ensuring user logout stage: %w", err)
	}

	redirectStage, err := inst.ensureRedirectStage(ctx, googleAccountLogoutRedirectStageName, googleAccountLogoutURL)
	if err != nil {
		return fmt.Errorf("ensuring Google logout redirect stage: %w", err)
	}

	for _, flow := range flows {
		if flow == nil {
			continue
		}
		if err := inst.ensureFlowStageBinding(ctx, flow.Slug, flow.PK, logoutStage.PK, 0); err != nil {
			return fmt.Errorf("binding user logout stage to %q: %w", flow.Slug, err)
		}
		if err := inst.ensureFlowStageBinding(ctx, flow.Slug, flow.PK, redirectStage.PK, 10); err != nil {
			return fmt.Errorf("binding Google logout redirect stage to %q: %w", flow.Slug, err)
		}
	}
	return nil
}

func (inst *Installer) ensureUserLogoutStage(ctx context.Context, name string) (*authentik.UserLogoutStage, error) {
	stage, err := inst.AK.GetUserLogoutStage(ctx, name)
	if err != nil {
		return nil, err
	}
	if stage != nil {
		return stage, nil
	}
	return inst.AK.CreateUserLogoutStage(ctx, name)
}

func (inst *Installer) ensureRedirectStage(ctx context.Context, name, targetURL string) (*authentik.RedirectStage, error) {
	params := authentik.RedirectStageParams{
		Name:         name,
		KeepContext:  false,
		Mode:         "static",
		TargetStatic: targetURL,
	}

	stage, err := inst.AK.GetRedirectStage(ctx, name)
	if err != nil {
		return nil, err
	}
	if stage == nil {
		return inst.AK.CreateRedirectStage(ctx, params)
	}
	if err := inst.AK.PatchRedirectStage(ctx, stage.PK, params); err != nil {
		return nil, err
	}
	return stage, nil
}

func (inst *Installer) ensureFlowStageBinding(ctx context.Context, flowSlug, flowPK, stagePK string, order int) error {
	existing, err := inst.AK.GetFlowStageBinding(ctx, flowSlug, stagePK)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil
	}
	return inst.AK.CreateFlowStageBinding(ctx, flowPK, stagePK, order)
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
