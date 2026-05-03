package cli

import (
	"context"
	"log/slog"
	"os"

	"github.com/caboose-ai/caboose-ai.io/internal/install"
)

func RunReset(ctx context.Context, inst *install.Installer) int {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	if inst.Config.DryRun {
		logger.Info("[dry-run] would stop all containers and remove volumes")
		logger.Info("[dry-run] would delete all bootstrap secrets from 1Password")
		logger.Info("[dry-run] would delete .env file")
		logger.Info("[dry-run] reset complete (no changes made)")
		return 0
	}

	logger.Info("resetting homelab — this will destroy all data and secrets")

	if err := inst.Reset(ctx, func(step, detail string) {
		logger.Info(detail, "step", step)
	}); err != nil {
		logger.Error("reset failed", "error", err)
		return 1
	}

	logger.Info("reset complete — run 'homelab install' to bootstrap from scratch")
	return 0
}
