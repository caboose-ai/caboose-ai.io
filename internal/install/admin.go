package install

import (
	"context"
	"fmt"
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
		return "", fmt.Errorf("generating recovery link: %w", err)
	}
	return link, nil
}

func (inst *Installer) ConfigureBrand(ctx context.Context) error {
	if inst.State.DryRun {
		return nil
	}

	flow, err := inst.AK.GetFlow(ctx, "default-recovery-flow")
	if err != nil {
		return fmt.Errorf("getting recovery flow: %w", err)
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
