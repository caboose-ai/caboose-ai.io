package servicebuilder

import (
	"context"
	"fmt"
	"strings"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
	"github.com/caboose-ai/caboose-ai.io/internal/docker"
	"github.com/caboose-ai/caboose-ai.io/internal/runner"
	"github.com/caboose-ai/caboose-ai.io/internal/secrets"
	"github.com/caboose-ai/caboose-ai.io/internal/service"
	"github.com/caboose-ai/caboose-ai.io/services/authentik"
	"github.com/caboose-ai/caboose-ai.io/services/forgejo"
	"github.com/caboose-ai/caboose-ai.io/services/grafana"
	"github.com/caboose-ai/caboose-ai.io/services/homarr"
	"github.com/caboose-ai/caboose-ai.io/services/mattermost"
	openwebui "github.com/caboose-ai/caboose-ai.io/services/open-webui"
	"github.com/caboose-ai/caboose-ai.io/services/paperclip"
	"github.com/caboose-ai/caboose-ai.io/services/portainer"
	"github.com/caboose-ai/caboose-ai.io/services/social"
	"github.com/caboose-ai/caboose-ai.io/services/sonarqube"
	"github.com/caboose-ai/caboose-ai.io/services/woodpecker"
)

type Dependencies struct {
	Config        *config.Config
	Secrets       secrets.SecretStore
	Runner        runner.CommandRunner
	HTTP          runner.HTTPClient
	DockerExec    *docker.ExecClient
	Authentik     *authentik.Client
	AllowDefaults bool
}

func Build(ctx context.Context, deps Dependencies) ([]service.ServiceConfigurator, error) {
	if deps.Authentik == nil {
		return nil, fmt.Errorf("Authentik client is required")
	}
	urls := deps.Config.URLs()

	getSecret := func(key string) (string, error) {
		value, err := deps.Secrets.Get(ctx, key)
		if err != nil && !deps.AllowDefaults {
			return "", fmt.Errorf("retrieving %s: %w", key, err)
		}
		return value, nil
	}

	giteaAdminPass, err := getSecret("GITEA_ADMIN_PASSWORD")
	if err != nil {
		return nil, err
	}
	portainerAdminPass, err := getSecret("PORTAINER_ADMIN_PASSWORD")
	if err != nil {
		return nil, err
	}
	sonarAdminPass, err := getSecret("SONAR_ADMIN_PASSWORD")
	if err != nil {
		return nil, err
	}
	if deps.AllowDefaults && portainerAdminPass == "" {
		portainerAdminPass = "admin"
	}

	portainerAPIURL := urls.Portainer
	homarrAPIURL := urls.Dashboard
	sonarAPIURL := urls.SonarQube
	if deps.Config.Orchestrator == "compose" {
		portainerAPIURL = "http://127.0.0.1:9000"
		homarrAPIURL = "http://127.0.0.1:7575"
		sonarAPIURL = "http://127.0.0.1:9005"
	}

	manifests, err := loadServiceManifests()
	if err != nil {
		manifests = fallbackDashboardManifests()
	}

	return []service.ServiceConfigurator{
		forgejo.New(deps.Authentik, deps.DockerExec, deps.Secrets, "forgejo", "auth-admin", urls.Authentik),
		woodpecker.New(deps.DockerExec, deps.HTTP, deps.Secrets, "woodpecker-server", "auth-admin", giteaAdminPass, urls.Woodpecker+"/authorize"),
		portainer.New(deps.Authentik, deps.HTTP, deps.Runner, portainerAPIURL, portainerAdminPass, deps.Config.Email, urls.Authentik, urls.Portainer+"/"),
		grafana.New(deps.Authentik, deps.Secrets),
		openwebui.New(deps.Authentik, deps.Secrets),
		homarr.New(deps.Authentik, deps.Secrets, homarrAPIURL, deps.HTTP, deps.DockerExec, "homarr", homarrApps(urls, manifests)...),
		paperclip.New(deps.Authentik, urls.Paperclip),
		sonarqube.New(sonarAPIURL, deps.HTTP, sonarAdminPass),
		mattermost.New(deps.DockerExec, deps.Secrets, "mattermost", deps.Config.Email),
		social.New(deps.Authentik, deps.Config.Social),
	}, nil
}

func loadServiceManifests() (map[string]service.Manifest, error) {
	root, err := service.FindManifestRoot(".")
	if err != nil {
		return nil, err
	}
	return service.LoadManifests(root)
}

func fallbackDashboardManifests() map[string]service.Manifest {
	show := func(value bool) *bool { return &value }
	return map[string]service.Manifest{
		"authentik":  {DisplayName: "Authentik", Dashboard: service.DashboardSpec{Show: show(true)}},
		"forgejo":    {DisplayName: "Forgejo", Dashboard: service.DashboardSpec{Show: show(true)}},
		"woodpecker": {DisplayName: "Woodpecker", Dashboard: service.DashboardSpec{Show: show(true)}},
		"portainer":  {DisplayName: "Portainer", Dashboard: service.DashboardSpec{Show: show(true)}},
		"grafana":    {DisplayName: "Grafana", Dashboard: service.DashboardSpec{Show: show(true)}},
		"open-webui": {DisplayName: "Open WebUI", Dashboard: service.DashboardSpec{Show: show(true), Path: "/oauth/oidc/login"}},
		"openclaw":   {DisplayName: "OpenClaw", Dashboard: service.DashboardSpec{Show: show(true)}},
		"homarr":     {DisplayName: "Homarr", Dashboard: service.DashboardSpec{Show: show(true)}},
		"paperclip":  {DisplayName: "Paperclip", Dashboard: service.DashboardSpec{Show: show(true)}},
		"ghost":      {DisplayName: "Ghost", Dashboard: service.DashboardSpec{Show: show(false)}},
		"mattermost": {DisplayName: "Mattermost", Dashboard: service.DashboardSpec{Show: show(false)}},
		"sonarqube":  {DisplayName: "SonarQube", Dashboard: service.DashboardSpec{Show: show(false)}},
	}
}

func homarrApps(urls config.URLs, manifests map[string]service.Manifest) []homarr.App {
	links := urls.ServiceLinks()
	apps := make([]homarr.App, 0, len(links))
	manifestsByName := make(map[string]service.Manifest, len(manifests))
	for _, manifest := range manifests {
		manifestsByName[manifest.DisplayName] = manifest
	}

	for _, link := range links {
		manifest, ok := manifestsByName[link.Name]
		if !ok || manifest.Dashboard.Show == nil || !*manifest.Dashboard.Show {
			continue
		}
		if manifest.Dashboard.Path != "" {
			link.URL = strings.TrimRight(link.URL, "/") + "/" + strings.TrimLeft(manifest.Dashboard.Path, "/")
		}
		apps = append(apps, homarr.App{Name: link.Name, URL: link.URL})
	}
	return apps
}

func LoadRegistry(ctx context.Context, manifestRoot string, configurators []service.ServiceConfigurator) (*service.Registry, error) {
	manifests, err := service.LoadManifests(manifestRoot)
	if err != nil {
		return nil, err
	}
	return service.NewRegistry(manifests, configurators), nil
}
