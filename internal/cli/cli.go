package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/caboose-ai/caboose-ai.io/internal/health"
	"github.com/caboose-ai/caboose-ai.io/internal/install"
)

func RunInstall(ctx context.Context, inst *install.Installer) int {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	dryRun := inst.Config.DryRun

	logger.Info("checking prerequisites")
	results, err := inst.CheckPrereqs(ctx)
	for _, r := range results {
		if r.Found {
			ver := strings.TrimSpace(r.Version)
			if len(ver) > 60 {
				ver = ver[:60]
			}
			logger.Info("  ✓ "+r.Name, "version", ver)
		} else {
			logger.Error("  ✗ "+r.Name, "error", r.Err)
		}
	}
	if err != nil {
		logger.Error("prerequisite check failed")
		return 1
	}
	logger.Info("prerequisites OK")

	if dryRun {
		logger.Info("[dry-run] would ensure 1Password vault", "vault", inst.Config.OPVault)
		logger.Info("[dry-run] would generate secrets")
		for _, def := range inst.State.BootstrapDefs() {
			if def.Prompt {
				logger.Info("  [dry-run] would prompt", "key", def.Key)
			} else {
				logger.Info("  [dry-run] would generate", "key", def.Key, "length", def.Length)
			}
		}
		logger.Info("[dry-run] would run docker compose up", "dir", inst.Config.ComposeDir)
		logger.Info("[dry-run] would wait for services to be healthy")
		logger.Info("[dry-run] would rename akadmin → auth-admin")
		logger.Info("[dry-run] would configure all services")
		logger.Info("[dry-run] install complete (no changes made)")
		return 0
	}

	logger.Info("ensuring 1Password vault", "vault", inst.Config.OPVault)
	if err := inst.EnsureVault(ctx); err != nil {
		logger.Error("vault setup failed", "error", err)
		return 2
	}
	logger.Info("vault ready")

	logger.Info("generating secrets")
	if err := inst.GenerateSecrets(ctx, func(key string) (string, error) {
		if key == "AUTHENTIK_BOOTSTRAP_EMAIL" && inst.Config.Email != "" {
			return inst.Config.Email, nil
		}
		if key == "N8N_USER" && inst.Config.N8NUser != "" {
			return inst.Config.N8NUser, nil
		}
		return "", fmt.Errorf("non-interactive mode requires %s in config file", key)
	}, func(p install.SecretProgress) {
		switch p.Action {
		case "generating":
			logger.Info("  ● generating", "key", p.Key)
		case "exists":
			logger.Info("  ✓ exists", "key", p.Key)
		case "prompting":
			logger.Info("  ? prompting", "key", p.Key)
		case "stored":
			logger.Info("  ✓ stored", "key", p.Key)
		case "error":
			logger.Error("  ✗ failed", "key", p.Key, "error", p.Err)
		}
	}); err != nil {
		logger.Error("secret generation failed", "error", err)
		return 2
	}
	logger.Info("secrets ready")

	logger.Info("starting docker compose", "dir", inst.Config.ComposeDir)
	if err := inst.ComposeUp(ctx); err != nil {
		logger.Error("compose up failed", "error", err)
		return 3
	}
	logger.Info("compose started")

	logger.Info("waiting for services to be healthy (timeout 120s)")
	for status := range inst.WaitHealthy(ctx) {
		if status.Healthy {
			logger.Info("  ✓ healthy", "service", status.Name, "elapsed", status.Elapsed)
		} else {
			logger.Warn("  ● polling", "service", status.Name, "error", status.Err)
		}
	}

	token, err := inst.Secrets.Get(ctx, "AUTHENTIK_BOOTSTRAP_TOKEN")
	if err != nil {
		logger.Error("retrieving Authentik bootstrap token", "error", err)
		return 3
	}
	inst.InitAK(token)

	logger.Info("building service configurators")
	if err := inst.BuildServices(ctx); err != nil {
		logger.Error("failed to build service configurators", "error", err)
		return 3
	}

	logger.Info("renaming admin user", "from", "akadmin", "to", "auth-admin")
	if err := inst.RenameAdmin(ctx); err != nil {
		logger.Error("admin rename failed", "error", err)
	} else {
		logger.Info("admin user renamed")
	}

	logger.Info("configuring services")
	svcResults := inst.ConfigureServices(ctx, func(name string, done bool) {
		if !done {
			logger.Info("  ● configuring", "service", name)
		}
	})

	hasErrors := false
	for _, r := range svcResults {
		if r.Err != nil {
			logger.Error("  ✗ failed", "service", r.Name, "error", r.Err)
			hasErrors = true
		} else if r.Result != nil {
			switch r.Result.Status.String() {
			case "skipped":
				logger.Warn("  – skipped", "service", r.Name, "reason", r.Result.Message)
			case "already configured":
				logger.Info("  ✓ already configured", "service", r.Name)
			default:
				logger.Info("  ✓ "+r.Result.Status.String(), "service", r.Name, "message", r.Result.Message)
			}
		}
	}

	if len(inst.State.RestartNeeded) > 0 {
		logger.Info("restarting services", "services", inst.State.RestartNeeded)
		if err := inst.RestartServices(ctx); err != nil {
			logger.Error("restart failed", "error", err)
		} else {
			logger.Info("services restarted")
		}
	}

	logger.Info("final health check")
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
		logger.Info("  ✓ healthy", "service", status.Name)
	} else {
		logger.Warn("  ● polling", "service", status.Name, "error", status.Err)
	}
}
