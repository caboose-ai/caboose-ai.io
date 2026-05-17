package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
	"github.com/caboose-ai/caboose-ai.io/internal/health"
	"github.com/caboose-ai/caboose-ai.io/internal/install"
)

func RunInstall(ctx context.Context, inst *install.Installer) int {
	console := NewConsole()
	dryRun := inst.Config.DryRun

	console.Banner("Homelab SSO Install", inst.Config.Domain)
	console.Phase("Prerequisites")
	console.Run("checking host tools")
	results, err := inst.CheckPrereqs(ctx)
	for _, r := range results {
		if r.Found {
			ver := strings.TrimSpace(r.Version)
			if len(ver) > 60 {
				ver = ver[:60]
			}
			console.Success(r.Name, "version", ver)
		} else {
			console.Error(r.Name, "error", r.Err)
		}
	}
	if err != nil {
		console.Error("prerequisite check failed")
		return 1
	}
	console.Success("prerequisites OK")

	if dryRun {
		console.Phase("Dry Run")
		console.Run("would ensure 1Password vaults", "static", inst.Config.OPStaticVault, "dynamic", inst.Config.OPVault)
		console.Run("would generate secrets")
		for _, def := range inst.State.BootstrapDefs() {
			if def.Prompt {
				console.Info("  ? would prompt", "key", def.Key)
			} else {
				console.Info("  ● would generate", "key", def.Key, "length", def.Length)
			}
		}
		console.Run("would apply homelab backend", "orchestrator", inst.Backend.Name(), "dir", inst.Config.ComposeDir)
		console.Run("would write runtime exposure config", "serve_mode", inst.Config.ServeMode, "bind", inst.Config.BindAddress())
		console.Run("would resolve optional social OAuth credentials from config/1Password")
		console.Run("would wait for services to be healthy")
		console.Run("would rename akadmin to auth-admin")
		console.Run("would configure Authentik brand recovery flow")
		console.Run("would generate admin recovery link")
		console.Run("would configure Turnstile captcha stage")
		console.Run("would configure inactive-user enrollment")
		console.Run("would initialize Forgejo admin user")
		console.Run("would provision OAuth2 providers")
		for _, spec := range inst.ProviderSpecs() {
			console.Info("  ● would ensure provider", "name", spec.Name, "slug", spec.Slug)
		}
		console.Run("would configure all services")
		console.Run("would start and seed Paperclip software-shop profile")
		console.Success("install dry run complete")
		return 0
	}

	console.Phase("Secrets")
	console.Run("ensuring 1Password vaults", "static", inst.Config.OPStaticVault, "dynamic", inst.Config.OPVault)
	if err := inst.EnsureVault(ctx); err != nil {
		console.Error("vault setup failed", "error", err)
		return 2
	}
	console.Success("vaults ready")

	console.Run("restoring static .env credentials from secret store")
	if err := inst.RestoreStaticEnv(ctx); err != nil {
		console.Error("static credential restore failed", "error", err)
		return 2
	}
	console.Success("static credentials restored")

	console.Run("generating and loading secrets")
	if err := inst.GenerateSecrets(ctx, func(key string) (string, error) {
		if key == "AUTHENTIK_BOOTSTRAP_EMAIL" && inst.Config.Email != "" {
			return inst.Config.Email, nil
		}
		return "", fmt.Errorf("non-interactive mode requires %s in config file", key)
	}, func(p install.SecretProgress) {
		switch p.Action {
		case "generating":
			console.Run("generating secret", "key", p.Key)
		case "exists":
			console.Success("secret loaded", "key", p.Key)
		case "prompting":
			console.Info("  ? prompting", "key", p.Key)
		case "stored":
			console.Success("secret stored", "key", p.Key)
		case "error":
			console.Error("secret failed", "key", p.Key, "error", p.Err)
		}
	}); err != nil {
		console.Error("secret generation failed", "error", err)
		return 2
	}
	console.Success("secrets ready")

	console.Run("resolving optional social OAuth credentials", "providers", "GitHub,Google")
	if err := inst.ResolveSocialCredentials(ctx, nil); err != nil {
		console.Error("social credential resolution failed", "error", err)
		return 2
	}
	console.Success("social credentials ready")

	console.Phase("Compose")
	console.Run("writing runtime exposure config", "serve_mode", inst.Config.ServeMode, "bind", inst.Config.BindAddress())
	if err := inst.WriteRuntimeEnv(ctx); err != nil {
		console.Error("runtime exposure config failed", "error", err)
		return 3
	}
	console.Success("runtime exposure config ready")

	console.Run("starting homelab backend", "orchestrator", inst.Backend.Name(), "dir", inst.Config.ComposeDir)
	if err := inst.ComposeUp(ctx); err != nil {
		console.Error("backend apply failed", "orchestrator", inst.Backend.Name(), "error", err)
		return 3
	}
	console.Success("backend started", "orchestrator", inst.Backend.Name())

	console.Run("waiting for services to be healthy", "timeout", "5m")
	healthOK := true
	for status := range inst.WaitHealthy(ctx) {
		if status.Healthy {
			console.Success("healthy", "service", status.Name, "elapsed", status.Elapsed)
		} else {
			console.Warn("polling", "service", status.Name, "error", status.Err)
			if status.Err == context.DeadlineExceeded {
				healthOK = false
			}
		}
	}
	if !healthOK {
		console.Error("health check timed out; one or more services did not become ready")
		return 3
	}

	console.Phase("Authentik")
	token, err := inst.Secrets.Get(ctx, "AUTHENTIK_BOOTSTRAP_TOKEN")
	if err != nil {
		console.Error("retrieving Authentik bootstrap token", "error", err)
		return 3
	}
	inst.InitAK(token)

	console.Phase("Forgejo")
	console.Run("initializing Forgejo admin user")
	if err := inst.InitForgejo(ctx); err != nil {
		console.Error("Forgejo init failed", "error", err)
		return 3
	}
	console.Success("Forgejo admin user ready")

	console.Phase("SSO Bootstrap")
	console.Run("renaming admin user", "from", "akadmin", "to", "auth-admin")
	if err := inst.RenameAdmin(ctx); err != nil {
		if install.IsAuthentikTokenError(err) {
			console.Warn("Authentik bootstrap token is invalid; recovering from running container")
			if recoverErr := inst.RecoverAuthentikBootstrapToken(ctx); recoverErr != nil {
				console.Error("admin token recovery failed", "error", recoverErr)
				return 3
			}
			err = inst.RenameAdmin(ctx)
		}
		if err != nil {
			console.Error("admin rename failed", "error", err)
		} else {
			console.Success("admin user renamed")
		}
	} else {
		console.Success("admin user renamed")
	}

	console.Run("configuring Authentik brand recovery flow")
	if err := inst.ConfigureBrand(ctx); err != nil {
		if install.IsAuthentikTokenError(err) {
			console.Warn("Authentik bootstrap token is invalid; recovering from running container")
			if recoverErr := inst.RecoverAuthentikBootstrapToken(ctx); recoverErr != nil {
				console.Error("admin token recovery failed", "error", recoverErr)
				return 3
			}
			err = inst.ConfigureBrand(ctx)
		}
		if err != nil {
			console.Error("brand recovery flow setup failed", "error", err)
			return 3
		}
	}
	console.Success("brand recovery flow ready")

	console.Run("generating admin recovery link")
	recoveryLink, err := inst.GenerateAdminRecoveryLink(ctx, nil)
	if err != nil {
		if install.IsAuthentikTokenError(err) {
			console.Warn("Authentik bootstrap token is invalid; recovering from running container")
			if recoverErr := inst.RecoverAuthentikBootstrapToken(ctx); recoverErr != nil {
				console.Error("admin token recovery failed", "error", recoverErr)
				return 3
			}
			recoveryLink, err = inst.GenerateAdminRecoveryLink(ctx, nil)
		}
		if err != nil {
			console.Error("admin recovery link generation failed", "error", err)
			return 3
		}
	}
	inst.State.AdminRecoveryLink = recoveryLink
	if recoveryLink != "" {
		console.Link("admin recovery link", recoveryLink)
	} else {
		console.Warn("admin recovery link unavailable")
	}

	console.Run("configuring Turnstile captcha stage")
	if err := inst.SetupCaptcha(ctx, nil); err != nil {
		console.Error("captcha setup failed", "error", err)
		return 3
	}
	console.Success("Turnstile captcha stage ready")

	console.Run("configuring inactive-user enrollment")
	if err := inst.SetupInactiveEnrollment(ctx, nil); err != nil {
		console.Error("enrollment setup failed", "error", err)
		return 3
	}
	console.Success("inactive-user enrollment ready")

	console.Phase("Providers")
	console.Run("provisioning OAuth2 providers in Authentik")
	if err := inst.ProvisionProviders(ctx, func(p install.ProviderProgress) {
		switch p.Action {
		case "exists":
			console.Success("provider exists", "provider", p.Name)
		case "creating":
			console.Run("creating provider", "provider", p.Name)
		case "created":
			console.Success("provider created", "provider", p.Name)
		case "error":
			console.Error("provider failed", "provider", p.Name, "error", p.Err)
		}
	}); err != nil {
		console.Error("provider provisioning failed", "error", err)
		return 3
	}
	console.Success("providers ready")

	console.Run("provisioning proxy providers and outpost in Authentik")
	if err := inst.ProvisionOutpost(ctx, func(p install.OutpostProgress) {
		switch p.Action {
		case "exists":
			console.Success("provider exists", "provider", p.Name)
		case "creating":
			console.Run("creating provider", "provider", p.Name)
		case "created":
			console.Success("provider created", "provider", p.Name)
		case "binding":
			console.Run("binding providers to outpost")
		case "bound":
			console.Success("outpost updated")
		case "error":
			console.Error("provider failed", "provider", p.Name, "error", p.Err)
		}
	}); err != nil {
		console.Error("outpost provisioning failed", "error", err)
		return 3
	}
	console.Success("outpost ready")

	console.Phase("Services")
	console.Run("building service configurators")
	if err := inst.BuildServices(ctx); err != nil {
		console.Error("failed to build service configurators", "error", err)
		return 3
	}

	console.Run("configuring downstream services")
	svcResults := inst.ConfigureServices(ctx, func(name string, done bool) {
		if !done {
			console.Run("configuring service", "service", name)
		}
	})

	hasErrors := false
	for _, r := range svcResults {
		if r.Err != nil {
			console.Error("service failed", "service", r.Name, "error", r.Err)
			hasErrors = true
		} else if r.Result != nil {
			switch r.Result.Status.String() {
			case "skipped":
				console.Skip("service skipped", "service", r.Name, "reason", r.Result.Message)
			case "already configured":
				console.Success("service already configured", "service", r.Name)
			default:
				console.Success("service "+r.Result.Status.String(), "service", r.Name, "message", r.Result.Message)
			}
		}
	}

	if len(inst.State.RestartNeeded) > 0 {
		console.Run("restarting services", "services", inst.State.RestartNeeded)
		if err := inst.RestartServices(ctx); err != nil {
			console.Error("restart failed", "error", err)
		} else {
			console.Success("services restarted")
		}
	}

	console.Phase("Paperclip")
	console.Run("starting and seeding Paperclip software-shop profile")
	report, err := inst.BootstrapPaperclip(ctx, inst.Paperclip)
	if err != nil {
		console.Error("Paperclip bootstrap failed", "error", err)
		return 4
	}
	console.Success("Paperclip seeded", "company", report.CompanyID, "created", formatSeedCounts(report.Created), "existing", formatSeedCounts(report.Existing))

	console.Phase("Final Health")
	console.Run("running final health check")
	finalHealthOK := true
	for status := range inst.WaitHealthy(ctx) {
		logHealth(console, status)
		if status.Err == context.DeadlineExceeded {
			finalHealthOK = false
		}
	}

	if !finalHealthOK {
		console.Warn("one or more services remain unhealthy after restart")
		return 4
	}

	if hasErrors {
		console.Warn("install completed with errors")
		return 4
	}
	console.Phase("Complete")
	console.Success("install complete")
	if inst.State.AdminRecoveryLink != "" {
		console.Link("admin recovery link", inst.State.AdminRecoveryLink)
	}
	logServiceLinks(console, inst.Config.URLs())
	return 0
}

func formatSeedCounts(counts map[string]int) string {
	if len(counts) == 0 {
		return "none"
	}
	parts := make([]string, 0, len(counts))
	for key, count := range counts {
		parts = append(parts, fmt.Sprintf("%s:%d", key, count))
	}
	return strings.Join(parts, ",")
}

func logServiceLinks(console *Console, urls config.URLs) {
	console.Info("Service URLs")
	for _, link := range urls.ServiceLinks() {
		console.Link(link.Name, link.URL)
	}
}

func logHealth(console *Console, status health.Status) {
	if status.Healthy {
		console.Success("healthy", "service", status.Name)
	} else {
		console.Warn("polling", "service", status.Name, "error", status.Err)
	}
}
