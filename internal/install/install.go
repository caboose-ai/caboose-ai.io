package install

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
	"github.com/caboose-ai/caboose-ai.io/internal/docker"
	"github.com/caboose-ai/caboose-ai.io/internal/health"
	"github.com/caboose-ai/caboose-ai.io/internal/prereq"
	"github.com/caboose-ai/caboose-ai.io/internal/runner"
	"github.com/caboose-ai/caboose-ai.io/internal/secrets"
	"github.com/caboose-ai/caboose-ai.io/internal/services"
	"github.com/caboose-ai/caboose-ai.io/internal/services/authentik"
	"github.com/caboose-ai/caboose-ai.io/internal/services/forgejo"
	"github.com/caboose-ai/caboose-ai.io/internal/services/grafana"
	"github.com/caboose-ai/caboose-ai.io/internal/services/mattermost"
	"github.com/caboose-ai/caboose-ai.io/internal/services/openwebui"
	"github.com/caboose-ai/caboose-ai.io/internal/services/portainer"
	"github.com/caboose-ai/caboose-ai.io/internal/services/social"
	"github.com/caboose-ai/caboose-ai.io/internal/services/woodpecker"
)

type Installer struct {
	Config     *config.Config
	State      *State
	Secrets    secrets.SecretStore
	Runner     runner.CommandRunner
	HTTP       runner.HTTPClient
	Compose    *docker.ComposeClient
	DockerExec *docker.ExecClient
	AK         *authentik.Client
	Services   []services.ServiceConfigurator
}

func New(cfg *config.Config, secretStore secrets.SecretStore, r runner.CommandRunner, httpClient runner.HTTPClient) *Installer {
	compose := docker.NewComposeClient(r, cfg.ComposeDir)
	dockerExec := docker.NewExecClient(r)

	inst := &Installer{
		Config:     cfg,
		State:      NewState(),
		Secrets:    secretStore,
		Runner:     r,
		HTTP:       httpClient,
		Compose:    compose,
		DockerExec: dockerExec,
	}

	inst.State.DryRun = cfg.DryRun
	inst.State.Force = cfg.Force
	inst.State.Domain = cfg.Domain
	inst.State.ComposeDir = cfg.ComposeDir

	return inst
}

func (inst *Installer) InitAK(token string) {
	urls := inst.Config.URLs()
	client := authentik.NewClient(urls.Authentik, token, inst.HTTP)
	if inst.Config.Verbose {
		client.Logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}
	inst.AK = client
}

// BuildServices constructs and registers all service configurators. It must be
// called after InitAK because most configurators depend on the Authentik client.
func (inst *Installer) BuildServices(ctx context.Context) error {
	if inst.AK == nil {
		return fmt.Errorf("BuildServices called before InitAK")
	}
	urls := inst.Config.URLs()

	giteaAdminPass, err := inst.Secrets.Get(ctx, "GITEA_ADMIN_PASSWORD")
	if err != nil {
		return fmt.Errorf("retrieving GITEA_ADMIN_PASSWORD: %w", err)
	}

	portainerAdminPass, err := inst.Secrets.Get(ctx, "PORTAINER_ADMIN_PASSWORD")
	if err != nil {
		return fmt.Errorf("retrieving PORTAINER_ADMIN_PASSWORD: %w", err)
	}

	inst.Services = []services.ServiceConfigurator{
		forgejo.New(inst.AK, inst.DockerExec, inst.Secrets, "forgejo", "auth-admin", urls.Authentik),
		woodpecker.New(inst.DockerExec, inst.HTTP, inst.Secrets, "woodpecker-server", "auth-admin", giteaAdminPass, urls.Woodpecker+"/authorize"),
		portainer.New(inst.AK, inst.HTTP, urls.Portainer, portainerAdminPass, urls.Authentik, urls.Portainer+"/"),
		grafana.New(inst.AK, inst.Secrets),
		openwebui.New(inst.AK, inst.Secrets),
		mattermost.New(inst.AK, inst.DockerExec, "mattermost", urls.Authentik),
		social.New(inst.AK, inst.Config.Social),
	}
	return nil
}

func (inst *Installer) CheckPrereqs(ctx context.Context) ([]prereq.Result, error) {
	checker := prereq.NewChecker(inst.Runner)
	results := checker.CheckAll(ctx)
	if !prereq.AllPassed(results) {
		var missing []string
		for _, r := range results {
			if !r.Found {
				missing = append(missing, r.Name)
			}
		}
		return results, fmt.Errorf("missing prerequisites: %v", missing)
	}
	return results, nil
}

func (inst *Installer) EnsureVault(ctx context.Context) error {
	return inst.Secrets.EnsureVault(ctx)
}

type SecretProgress struct {
	Key    string
	Action string // "generating", "exists", "prompted", "stored", "error"
	Err    error
}

func (inst *Installer) GenerateSecrets(ctx context.Context, promptFn func(key string) (string, error), progressFn func(SecretProgress)) error {
	if progressFn == nil {
		progressFn = func(SecretProgress) {}
	}
	for _, def := range secrets.BootstrapSecrets() {
		if def.Prompt {
			progressFn(SecretProgress{Key: def.Key, Action: "prompting"})
			val, err := promptFn(def.Key)
			if err != nil {
				progressFn(SecretProgress{Key: def.Key, Action: "error", Err: err})
				return err
			}
			if val == "" {
				err := fmt.Errorf("required value for %s cannot be empty", def.Key)
				progressFn(SecretProgress{Key: def.Key, Action: "error", Err: err})
				return err
			}
			if err := inst.Secrets.Put(ctx, def.Key, val); err != nil {
				progressFn(SecretProgress{Key: def.Key, Action: "error", Err: err})
				return err
			}
			progressFn(SecretProgress{Key: def.Key, Action: "stored"})
			continue
		}

		existing, _ := inst.Secrets.Get(ctx, def.Key)
		if existing != "" && !inst.State.Force {
			progressFn(SecretProgress{Key: def.Key, Action: "exists"})
			continue
		}

		progressFn(SecretProgress{Key: def.Key, Action: "generating"})
		_, err := inst.Secrets.Generate(ctx, def.Key, def.Length, def.Opts)
		if err != nil {
			progressFn(SecretProgress{Key: def.Key, Action: "error", Err: err})
			return fmt.Errorf("generating %s: %w", def.Key, err)
		}
		progressFn(SecretProgress{Key: def.Key, Action: "stored"})
	}
	return nil
}

func (inst *Installer) ComposeUp(ctx context.Context) error {
	if inst.State.DryRun {
		return nil
	}
	_, err := inst.Compose.Up(ctx)
	return err
}

func (inst *Installer) WaitHealthy(ctx context.Context) <-chan health.Status {
	token, _ := inst.Secrets.Get(ctx, "AUTHENTIK_BOOTSTRAP_TOKEN")
	urls := inst.Config.URLs()

	checks := []health.Checker{
		&health.HTTPChecker{
			ServiceName: "Authentik",
			URL:         urls.Authentik + "/api/v3/root/config/",
			Client:      inst.HTTP,
			Headers:     map[string]string{"Authorization": "Bearer " + token},
		},
		&health.HTTPChecker{
			ServiceName: "Forgejo",
			URL:         "http://127.0.0.1:3000/api/v1/settings/api",
			Client:      inst.HTTP,
		},
	}

	poller := health.NewPoller(checks, 3*time.Second, 5*time.Minute)
	return poller.PollAll(ctx)
}

func (inst *Installer) RenameAdmin(ctx context.Context) error {
	if inst.AK == nil {
		return fmt.Errorf("Authentik client not initialized")
	}
	if inst.State.DryRun {
		return nil
	}
	return inst.AK.RenameAdminUser(ctx, "akadmin", "auth-admin")
}

func (inst *Installer) ConfigureServices(ctx context.Context, progressFn func(name string, done bool)) []ServiceResult {
	if progressFn == nil {
		progressFn = func(string, bool) {}
	}
	opts := services.ConfigureOpts{
		DryRun: inst.State.DryRun,
		Force:  inst.State.Force,
	}

	for _, svc := range inst.Services {
		progressFn(svc.Name(), false)
		result, err := svc.Configure(ctx, opts)
		inst.State.AddServiceResult(svc.Name(), result, err)
		progressFn(svc.Name(), true)
	}
	return inst.State.ServiceResults
}

func (inst *Installer) Reset(ctx context.Context, progressFn func(step, detail string)) error {
	if progressFn == nil {
		progressFn = func(string, string) {}
	}

	progressFn("compose", "stopping containers and removing volumes")
	if _, err := inst.Compose.DownWithVolumes(ctx); err != nil {
		return fmt.Errorf("compose down -v: %w", err)
	}

	progressFn("secrets", "deleting bootstrap secrets from 1Password")
	for _, def := range secrets.BootstrapSecrets() {
		if err := inst.Secrets.Delete(ctx, def.Key); err != nil {
			progressFn("secrets", fmt.Sprintf("warning: could not delete %s: %v", def.Key, err))
		}
	}

	progressFn("secrets", "deleting derived OAuth secrets from 1Password")
	for _, key := range secrets.DerivedSecretKeys() {
		if err := inst.Secrets.Delete(ctx, key); err != nil {
			progressFn("secrets", fmt.Sprintf("warning: could not delete %s: %v", key, err))
		}
	}

	progressFn("env", "removing .env file")
	envPath := inst.Config.EnvPath()
	if err := os.Remove(envPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing %s: %w", envPath, err)
	}

	progressFn("done", "reset complete — run 'homelab install' to start fresh")
	return nil
}

func (inst *Installer) RestartServices(ctx context.Context) error {
	if inst.State.DryRun || len(inst.State.RestartNeeded) == 0 {
		return nil
	}

	unique := make(map[string]bool)
	var svcs []string
	for _, s := range inst.State.RestartNeeded {
		if !unique[s] {
			unique[s] = true
			svcs = append(svcs, s)
		}
	}

	_, err := inst.Compose.Restart(ctx, svcs...)
	return err
}
