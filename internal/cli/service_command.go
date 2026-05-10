package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
	"github.com/caboose-ai/caboose-ai.io/internal/docker"
	"github.com/caboose-ai/caboose-ai.io/internal/install"
	"github.com/caboose-ai/caboose-ai.io/internal/runner"
	"github.com/caboose-ai/caboose-ai.io/internal/secrets"
	"github.com/caboose-ai/caboose-ai.io/internal/service"
	"github.com/caboose-ai/caboose-ai.io/internal/servicebuilder"
)

type ServiceCommandOptions struct {
	DryRun bool
	Force  bool
}

func RunServiceCommand(ctx context.Context, cfg *config.Config, r runner.CommandRunner, secretStore secrets.SecretStore, args []string, opts ServiceCommandOptions) int {
	args, opts = normalizeServiceArgs(args, opts)
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: homelab service <slug> <status|configure|logs|smoke|open>")
		return 1
	}

	slug := args[0]
	action := args[1]
	if isServiceAction(args[0]) {
		action = args[0]
		slug = args[1]
	}

	root, err := service.FindManifestRoot(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "service registry error: %v\n", err)
		return 1
	}
	manifests, err := service.LoadManifests(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "service registry error: %v\n", err)
		return 1
	}
	manifest, ok := manifests[slug]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown service %q\n", slug)
		return 1
	}

	compose := docker.NewComposeClient(r, cfg.ComposeDir)
	switch action {
	case "status":
		return runServiceStatus(ctx, compose, runner.NewHTTPClient(), cfg.URLs(), manifest)
	case "logs":
		return runServiceLogs(ctx, compose, manifest)
	case "open":
		return runServiceOpen(cfg, manifest)
	case "smoke":
		return runServiceSmoke(ctx, r, manifest)
	case "configure":
		return runServiceConfigure(ctx, cfg, r, secretStore, slug, opts)
	default:
		fmt.Fprintf(os.Stderr, "unknown service action %q\n", action)
		return 1
	}
}

func normalizeServiceArgs(args []string, opts ServiceCommandOptions) ([]string, ServiceCommandOptions) {
	filtered := make([]string, 0, len(args))
	for _, arg := range args {
		switch arg {
		case "--dry-run":
			opts.DryRun = true
		case "--force":
			opts.Force = true
		default:
			filtered = append(filtered, arg)
		}
	}
	return filtered, opts
}

func isServiceAction(value string) bool {
	switch value {
	case "status", "configure", "logs", "smoke", "open":
		return true
	default:
		return false
	}
}

func runServiceStatus(ctx context.Context, compose *docker.ComposeClient, httpClient runner.HTTPClient, urls config.URLs, manifest service.Manifest) int {
	status := map[string]any{
		"service":          manifest.Slug,
		"display_name":     manifest.DisplayName,
		"runtime":          manifest.Runtime,
		"compose_services": manifest.ComposeServices,
	}
	if manifest.Runtime == service.RuntimeExternal {
		status["status"] = "external_no_local_container"
		status["message"] = "external service has no local compose container"
		if healthURL := serviceHealthURL(urls, manifest); healthURL != "" {
			status["health_url"] = healthURL
			healthy, err := checkServiceHealth(ctx, httpClient, healthURL)
			status["healthy"] = healthy
			if err != nil {
				status["health_error"] = err.Error()
			}
		}
		data, _ := json.MarshalIndent(status, "", "  ")
		fmt.Println(string(data))
		return 0
	}
	running := map[string]bool{}
	for _, composeService := range manifest.ComposeServices {
		ok, err := compose.IsRunning(ctx, composeService)
		if err != nil {
			fmt.Fprintf(os.Stderr, "status failed for %s: %v\n", composeService, err)
			return 2
		}
		running[composeService] = ok
	}
	status["running"] = running
	data, _ := json.MarshalIndent(status, "", "  ")
	fmt.Println(string(data))
	return 0
}

func runServiceLogs(ctx context.Context, compose *docker.ComposeClient, manifest service.Manifest) int {
	if manifest.Runtime == service.RuntimeExternal {
		fmt.Printf("service %q uses external runtime; no compose logs are available\n", manifest.Slug)
		return 0
	}
	if len(manifest.ComposeServices) == 0 {
		fmt.Fprintf(os.Stderr, "service %q has no compose services\n", manifest.Slug)
		return 1
	}
	for _, composeService := range manifest.ComposeServices {
		out, err := compose.Logs(ctx, composeService, 100)
		if err != nil {
			fmt.Fprintf(os.Stderr, "logs failed for %s: %v\n", composeService, err)
			return 2
		}
		fmt.Printf("==> %s\n%s", composeService, string(out))
	}
	return 0
}

func serviceHealthURL(urls config.URLs, manifest service.Manifest) string {
	key := manifest.Health.URLKey
	if key == "" {
		key = manifest.URLKey
	}
	base := serviceURL(urls, key)
	if base == "" {
		return ""
	}
	path := manifest.Health.Path
	if path == "" {
		path = "/"
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/")
}

func checkServiceHealth(ctx context.Context, client runner.HTTPClient, healthURL string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return false, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return false, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return true, nil
}

func runServiceOpen(cfg *config.Config, manifest service.Manifest) int {
	url := serviceURL(cfg.URLs(), manifest.URLKey)
	if url == "" {
		fmt.Fprintf(os.Stderr, "service %q does not have a registered URL\n", manifest.Slug)
		return 1
	}
	fmt.Println(url)
	return 0
}

func runServiceSmoke(ctx context.Context, r runner.CommandRunner, manifest service.Manifest) int {
	if manifest.SmokeFlow == "" {
		fmt.Fprintf(os.Stderr, "service %q does not define a smoke flow\n", manifest.Slug)
		return 1
	}
	out, err := r.Run(ctx, "go", "test", "-tags", "integration", "./internal/smoketest/", "-run", "TestSSO_ServiceSmoke", "-v", "-args", "-smoke-flow", manifest.SmokeFlow)
	if len(out) > 0 {
		fmt.Print(string(out))
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "smoke failed for %s: %v\n", manifest.Slug, err)
		return 2
	}
	return 0
}

func runServiceConfigure(ctx context.Context, cfg *config.Config, r runner.CommandRunner, secretStore secrets.SecretStore, slug string, opts ServiceCommandOptions) int {
	inst := install.New(cfg, secretStore, r, runner.NewHTTPClient())
	token, _ := secretStore.Get(ctx, "AUTHENTIK_BOOTSTRAP_TOKEN")
	if token == "" && opts.DryRun {
		token = "dry-run"
	}
	if token == "" {
		fmt.Fprintln(os.Stderr, "AUTHENTIK_BOOTSTRAP_TOKEN is required to configure services")
		return 2
	}
	inst.InitAK(token)
	configurators, err := servicebuilder.Build(ctx, servicebuilder.Dependencies{
		Config:        cfg,
		Secrets:       secretStore,
		Runner:        r,
		HTTP:          inst.HTTP,
		DockerExec:    inst.DockerExec,
		Authentik:     inst.AK,
		AllowDefaults: opts.DryRun,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "building service configurators: %v\n", err)
		return 2
	}
	registry := service.NewRegistry(map[string]service.Manifest{}, configurators)
	configurator, ok := registry.Configurator(slug)
	if !ok {
		fmt.Fprintf(os.Stderr, "service %q does not have a configurator\n", slug)
		return 1
	}
	result, err := configurator.Configure(ctx, service.ConfigureOpts{DryRun: opts.DryRun, Force: opts.Force})
	if err != nil {
		fmt.Fprintf(os.Stderr, "configure failed for %s: %v\n", slug, err)
		return 2
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
	return 0
}

func serviceURL(urls config.URLs, key string) string {
	switch strings.ToLower(key) {
	case "authentik":
		return urls.Authentik
	case "forgejo":
		return urls.Forgejo
	case "woodpecker":
		return urls.Woodpecker
	case "portainer":
		return urls.Portainer
	case "grafana":
		return urls.Grafana
	case "open_webui":
		return urls.OpenWebUI
	case "mattermost":
		return urls.Mattermost
	case "dashboard":
		return urls.Dashboard
	case "dash_alias":
		return urls.DashAlias
	case "openclaw":
		return urls.OpenClaw
	case "sonarqube":
		return urls.SonarQube
	case "ghost":
		return urls.Ghost
	case "paperclip":
		return urls.Paperclip
	case "ci":
		return urls.CI
	default:
		return ""
	}
}
