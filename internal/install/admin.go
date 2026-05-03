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
