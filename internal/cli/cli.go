package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/caboose-ai/caboose-ai.io/internal/health"
	"github.com/caboose-ai/caboose-ai.io/internal/install"
)

func RunInstall(ctx context.Context, inst *install.Installer) int {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	logger.Info("checking prerequisites")
	results, err := inst.CheckPrereqs(ctx)
	if err != nil {
		logger.Error("prerequisite check failed", "error", err)
		for _, r := range results {
			if !r.Found {
				logger.Error("missing", "tool", r.Name)
			}
		}
		return 1
	}
	logger.Info("prerequisites OK")

	logger.Info("ensuring 1Password vault")
	if err := inst.EnsureVault(ctx); err != nil {
		logger.Error("vault setup failed", "error", err)
		return 2
	}

	logger.Info("generating secrets")
	if err := inst.GenerateSecrets(ctx, func(key string) (string, error) {
		if key == "AUTHENTIK_BOOTSTRAP_EMAIL" && inst.Config.Email != "" {
			return inst.Config.Email, nil
		}
		if key == "N8N_USER" && inst.Config.N8NUser != "" {
			return inst.Config.N8NUser, nil
		}
		return "", fmt.Errorf("non-interactive mode requires %s in config file", key)
	}); err != nil {
		logger.Error("secret generation failed", "error", err)
		return 2
	}
	logger.Info("secrets generated")

	logger.Info("starting docker compose")
	if err := inst.ComposeUp(ctx); err != nil {
		logger.Error("compose up failed", "error", err)
		return 3
	}

	logger.Info("waiting for services to be healthy")
	for status := range inst.WaitHealthy(ctx) {
		if status.Healthy {
			logger.Info("healthy", "service", status.Name, "elapsed", status.Elapsed)
		} else {
			logger.Warn("not ready", "service", status.Name, "error", status.Err)
		}
	}

	token, _ := inst.Secrets.Get(ctx, "AUTHENTIK_BOOTSTRAP_TOKEN")
	inst.InitAK(token)

	logger.Info("renaming admin user", "from", "akadmin", "to", "auth-admin")
	if err := inst.RenameAdmin(ctx); err != nil {
		logger.Error("admin rename failed", "error", err)
	}

	logger.Info("configuring services")
	svcResults := inst.ConfigureServices(ctx)
	hasErrors := false
	for _, r := range svcResults {
		if r.Err != nil {
			logger.Error("service config failed", "service", r.Name, "error", r.Err)
			hasErrors = true
		} else if r.Result != nil {
			level := slog.LevelInfo
			if r.Result.Status.String() == "skipped" {
				level = slog.LevelWarn
			}
			logger.Log(ctx, level, "service configured",
				"service", r.Name,
				"status", r.Result.Status.String(),
				"message", r.Result.Message,
			)
		}
	}

	logger.Info("restarting services with updated config")
	if err := inst.RestartServices(ctx); err != nil {
		logger.Error("restart failed", "error", err)
	}

	logger.Info("waiting for final health check")
	for status := range inst.WaitHealthy(ctx) {
		logHealth(logger, status)
	}

	if hasErrors {
		logger.Warn("install completed with errors")
		return 4
	}
	logger.Info("install complete")
	return 0
}

func logHealth(logger *slog.Logger, status health.Status) {
	if status.Healthy {
		logger.Info("healthy", "service", status.Name)
	} else {
		logger.Warn("unhealthy", "service", status.Name, "error", status.Err)
	}
}
