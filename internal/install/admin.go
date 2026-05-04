package install

import (
	"context"
	"fmt"

	"github.com/caboose-ai/caboose-ai.io/internal/services/authentik"
)

func (inst *Installer) GenerateAdminRecoveryLink(ctx context.Context, progressFn func(string)) (string, error) {
	if progressFn == nil {
		progressFn = func(string) {}
	}
	if inst.State.DryRun {
		return "https://auth." + inst.Config.Domain + "/if/flow/default-recovery-flow/?token=dry-run", nil
	}
	progressFn("Generating admin recovery link...")
	user, err := inst.AK.FindUser(ctx, "auth-admin")
	if err != nil {
		return "", fmt.Errorf("finding auth-admin user: %w", err)
	}
	if user == nil {
		return "", fmt.Errorf("auth-admin user not found")
	}
	link, err := inst.AK.GenerateRecoveryLink(ctx, user.PK)
	if err != nil {
		return "", nil
	}
	return link, nil
}

func (inst *Installer) ConfigureBrand(ctx context.Context) error {
	if inst.State.DryRun {
		return nil
	}

	flow, err := inst.AK.GetFlowByDesignation(ctx, "recovery")
	if err != nil {
		flow, err = inst.ensureRecoveryFlow(ctx)
		if err != nil {
			return fmt.Errorf("ensuring recovery flow: %w", err)
		}
	}

	brand, err := inst.AK.GetDefaultBrand(ctx)
	if err != nil {
		return fmt.Errorf("getting default brand: %w", err)
	}

	if err := inst.AK.SetBrandRecoveryFlow(ctx, brand.BrandUUID, flow.PK); err != nil {
		return fmt.Errorf("setting recovery flow: %w", err)
	}
	return nil
}

func (inst *Installer) ensureRecoveryFlow(ctx context.Context) (*authentik.Flow, error) {
	flow, err := inst.AK.CreateFlow(ctx, authentik.CreateFlowParams{
		Name:        "Default recovery flow",
		Slug:        "default-recovery-flow",
		Title:       "Reset your password",
		Designation: "recovery",
	})
	if err != nil {
		return nil, err
	}

	passwordChangeFlow, err := inst.AK.GetFlow(ctx, "default-password-change")
	if err != nil {
		return nil, fmt.Errorf("getting default-password-change flow for stage reuse: %w", err)
	}

	bindings, err := inst.AK.ListFlowStageBindings(ctx, passwordChangeFlow.Slug)
	if err != nil {
		return nil, fmt.Errorf("listing default-password-change bindings: %w", err)
	}

	for _, b := range bindings {
		if err := inst.AK.CreateFlowStageBinding(ctx, flow.PK, b.StageObj.PK, b.Order); err != nil {
			return nil, fmt.Errorf("binding stage %q to recovery flow: %w", b.StageObj.Name, err)
		}
	}

	return flow, nil
}
