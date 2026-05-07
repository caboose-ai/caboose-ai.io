package cli

import (
	"context"

	"github.com/caboose-ai/caboose-ai.io/internal/install"
)

func RunReset(ctx context.Context, inst *install.Installer) int {
	console := NewConsole()
	console.Banner("Homelab Reset", inst.Config.Domain)

	if inst.Config.DryRun {
		console.Phase("Dry Run")
		console.Run("would stop all containers and remove volumes")
		console.Run("would delete all bootstrap and derived OAuth secrets from 1Password")
		console.Run("would delete .env file")
		console.Success("reset dry run complete")
		return 0
	}

	console.Phase("Reset")
	console.Warn("resetting homelab; this will destroy all data and dynamic secrets")

	if err := inst.Reset(ctx, func(step, detail string) {
		console.Run(detail, "step", step)
	}); err != nil {
		console.Error("reset failed", "error", err)
		return 1
	}

	console.Phase("Complete")
	console.Success("reset complete; run 'homelab install' to bootstrap from scratch")
	return 0
}
