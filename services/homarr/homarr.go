package homarr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/caboose-ai/caboose-ai.io/internal/docker"
	"github.com/caboose-ai/caboose-ai.io/internal/runner"
	"github.com/caboose-ai/caboose-ai.io/internal/secrets"
	"github.com/caboose-ai/caboose-ai.io/internal/service"
	"github.com/caboose-ai/caboose-ai.io/services/authentik"
)

type Configurator struct {
	AK        *authentik.Client
	Secrets   secrets.SecretStore
	URL       string
	HTTP      runner.HTTPClient
	Docker    *docker.ExecClient
	Container string
	Apps      []App
}

type App struct {
	Name string
	URL  string
}

var dashboardIconSlugs = map[string]string{
	"Authentik":  "authentik",
	"Forgejo":    "forgejo",
	"Grafana":    "grafana",
	"Homarr":     "homarr",
	"Mattermost": "mattermost",
	"Open WebUI": "open-webui",
	"OpenClaw":   "openclaw",
	"Paperclip":  "paperclip",
	"Portainer":  "portainer",
	"SonarQube":  "sonarqube",
	"Woodpecker": "woodpecker-ci",
}

type seededApp struct {
	ID      string `json:"id"`
	ItemID  string `json:"itemId"`
	Name    string `json:"name"`
	Href    string `json:"href"`
	IconURL string `json:"iconUrl"`
}

func New(ak *authentik.Client, s secrets.SecretStore, url string, httpClient runner.HTTPClient, dockerExec *docker.ExecClient, container string, apps ...App) *Configurator {
	return &Configurator{
		AK:        ak,
		Secrets:   s,
		URL:       url,
		HTTP:      httpClient,
		Docker:    dockerExec,
		Container: container,
		Apps:      apps,
	}
}

func (c *Configurator) Name() string { return "Homarr OIDC" }
func (c *Configurator) Slug() string { return "homarr" }

func (c *Configurator) CheckConfigured(ctx context.Context) (bool, error) {
	id, err := c.Secrets.Get(ctx, "HOMARR_OIDC_CLIENT_ID")
	if err != nil {
		return false, err
	}
	return id != "", nil
}

func (c *Configurator) Configure(ctx context.Context, opts service.ConfigureOpts) (*service.ConfigureResult, error) {
	provider, err := c.AK.GetProvider(ctx, "homarr")
	if err != nil {
		return nil, fmt.Errorf("fetching Homarr provider from Authentik: %w", err)
	}

	if opts.DryRun {
		return &service.ConfigureResult{Status: service.StatusDryRun, Message: "would write Homarr OIDC credentials to .env"}, nil
	}

	existingID, _ := c.Secrets.Get(ctx, "HOMARR_OIDC_CLIENT_ID")
	if existingID == provider.ClientID && !opts.Force {
		if err := c.ensureInitialized(ctx); err != nil {
			return nil, err
		}
		return &service.ConfigureResult{Status: service.StatusAlreadyConfigured, Message: "Homarr OIDC already configured"}, nil
	}

	if err := c.Secrets.Put(ctx, "HOMARR_OIDC_CLIENT_ID", provider.ClientID); err != nil {
		return nil, err
	}
	if err := c.Secrets.Put(ctx, "HOMARR_OIDC_CLIENT_SECRET", provider.ClientSecret); err != nil {
		return nil, err
	}
	if err := c.ensureInitialized(ctx); err != nil {
		return nil, err
	}

	return &service.ConfigureResult{
		Status:          service.StatusCreated,
		Message:         "Homarr OIDC credentials written and first-run state initialized",
		RestartRequired: true,
		Services:        []string{"homarr"},
	}, nil
}

func (c *Configurator) ensureInitialized(ctx context.Context) error {
	if c.HTTP == nil || c.URL == "" {
		return nil
	}
	for i := 0; i < 5; i++ {
		step, err := c.currentStep(ctx)
		if err != nil {
			return err
		}
		switch step {
		case "start":
			if err := c.trpcPost(ctx, "onboard.nextStep", map[string]any{"preferredStep": "group"}); err != nil {
				return err
			}
		case "group":
			if err := c.trpcPost(ctx, "group.createInitialExternalGroup", map[string]any{"name": "authentik Admins"}); err != nil {
				return err
			}
		case "settings":
			if err := c.trpcPost(ctx, "serverSettings.initSettings", map[string]any{
				"analytics": map[string]any{
					"enableGeneral":         true,
					"enableWidgetData":      false,
					"enableIntegrationData": false,
					"enableUserData":        false,
				},
				"crawlingAndIndexing": map[string]any{
					"noIndex":              true,
					"noFollow":             true,
					"noTranslate":          true,
					"noSiteLinksSearchBox": false,
				},
			}); err != nil {
				return err
			}
		case "finish":
			return c.ensureDefaultBoard(ctx)
		default:
			return fmt.Errorf("unexpected Homarr onboarding step %q", step)
		}
	}
	return fmt.Errorf("Homarr onboarding did not reach finish")
}

func (c *Configurator) currentStep(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL+"/api/trpc/onboard.currentStep", nil)
	if err != nil {
		return "", fmt.Errorf("building Homarr onboarding request: %w", err)
	}
	req.Header.Set("x-trpc-source", "homelab")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("Homarr onboarding request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("Homarr onboarding returned HTTP %d: %s", resp.StatusCode, string(body))
	}
	var parsed struct {
		Result struct {
			Data struct {
				JSON struct {
					Current string `json:"current"`
				} `json:"json"`
			} `json:"data"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("parsing Homarr onboarding response: %w", err)
	}
	return parsed.Result.Data.JSON.Current, nil
}

func (c *Configurator) trpcPost(ctx context.Context, path string, payload map[string]any) error {
	body, _ := json.Marshal(map[string]any{"json": payload})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL+"/api/trpc/"+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("building Homarr %s request: %w", path, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-trpc-source", "homelab")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("Homarr %s request: %w", path, err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Homarr %s returned HTTP %d: %s", path, resp.StatusCode, string(respBody))
	}
	return nil
}

func (c *Configurator) ensureDefaultBoard(ctx context.Context) error {
	if c.Docker == nil || c.Container == "" {
		return nil
	}
	appsJSON, err := json.Marshal(seedApps(c.Apps))
	if err != nil {
		return fmt.Errorf("building Homarr app seed data: %w", err)
	}
	script := `
const Database = require("better-sqlite3");
const db = new Database("/appdata/db/db.sqlite");
const boardId = "homelab_default_board";
const sectionId = "homelab_default_section";
const apps = __HOMELAB_APPS__;
const boardGroups = [
  {name: "everyone", permission: "view"},
  {name: "authentik Admins", permission: "full"},
];
const layouts = [
  {id: "homelab_default_layout", name: "Mobile", columns: 2, breakpoint: 0, perRow: 1},
  {id: "homelab_tablet_layout", name: "Tablet", columns: 6, breakpoint: 768, perRow: 3},
  {id: "homelab_desktop_layout", name: "Desktop", columns: 12, breakpoint: 1200, perRow: 6},
];
const allowedAppIds = new Set(apps.map((app) => app.id));
const allowedItemIds = new Set(apps.map((app) => app.itemId));
const boardSetting = {json: {homeBoardId: boardId, mobileHomeBoardId: boardId, enableStatusByDefault: true, forceDisableStatus: false}};
db.transaction(() => {
  if (!db.prepare("select id from board where id = ?").get(boardId)) {
    db.prepare("insert into board (id, name, is_public, creator_id) values (?, ?, ?, null)").run(boardId, "Home", 0);
    db.prepare("insert into section (id, board_id, kind, x_offset, y_offset) values (?, ?, ?, ?, ?)").run(sectionId, boardId, "empty", 0, 0);
  }
  db.prepare("update board set is_public = 0 where id = ?").run(boardId);
  const upsertLayout = db.prepare("insert into layout (id, name, board_id, column_count, breakpoint) values (?, ?, ?, ?, ?) on conflict(id) do update set name = excluded.name, board_id = excluded.board_id, column_count = excluded.column_count, breakpoint = excluded.breakpoint");
  const upsertItemLayout = db.prepare("insert into item_layout (item_id, section_id, layout_id, x_offset, y_offset, width, height) values (?, ?, ?, ?, ?, ?, ?) on conflict(item_id, section_id, layout_id) do update set x_offset = excluded.x_offset, y_offset = excluded.y_offset, width = excluded.width, height = excluded.height");
  const upsertBoardGroupPermission = db.prepare("insert into boardGroupPermission (board_id, group_id, permission) values (?, ?, ?) on conflict(board_id, group_id, permission) do nothing");
  for (const layout of layouts) {
    upsertLayout.run(layout.id, layout.name, boardId, layout.columns, layout.breakpoint);
  }
  db.prepare("update serverSetting set value = ? where setting_key = ?").run(JSON.stringify(boardSetting), "board");
  db.prepare("update \"group\" set home_board_id = ?, mobile_home_board_id = ? where name in (?, ?)").run(boardId, boardId, "everyone", "authentik Admins");
  for (const group of boardGroups) {
    const row = db.prepare("select id from \"group\" where name = ?").get(group.name);
    if (row) {
      upsertBoardGroupPermission.run(boardId, row.id, group.permission);
    }
  }
  for (const row of db.prepare("select id from item where id like 'homelab_item_%'").all()) {
    if (!allowedItemIds.has(row.id)) {
      db.prepare("delete from item_layout where item_id = ?").run(row.id);
      db.prepare("delete from item where id = ?").run(row.id);
    }
  }
  for (const row of db.prepare("select id from app where id like 'homelab_app_%'").all()) {
    if (!allowedAppIds.has(row.id)) {
      db.prepare("delete from app where id = ?").run(row.id);
    }
  }
  for (const app of apps) {
    if (!db.prepare("select id from app where id = ?").get(app.id)) {
      db.prepare("insert into app (id, name, description, icon_url, href, ping_url) values (?, ?, null, ?, ?, ?)").run(app.id, app.name, app.iconUrl, app.href, app.href);
    } else {
      db.prepare("update app set name = ?, icon_url = ?, href = ?, ping_url = ? where id = ?").run(app.name, app.iconUrl, app.href, app.href, app.id);
    }
    if (!db.prepare("select id from item where id = ?").get(app.itemId)) {
      const options = {json: {appId: app.id, openInNewTab: true, pingEnabled: true, showTitle: true, layout: "bottom", descriptionDisplayMode: "hidden"}};
      db.prepare("insert into item (id, board_id, kind, options, advanced_options) values (?, ?, 'app', ?, ?)").run(app.itemId, boardId, JSON.stringify(options), JSON.stringify({json: {}}));
    }
  }
  for (const layout of layouts) {
    apps.forEach((app, i) => {
      upsertItemLayout.run(app.itemId, sectionId, layout.id, (i % layout.perRow) * 2, Math.floor(i / layout.perRow), 2, 1);
    });
  }
})();
`
	script = strings.Replace(script, "__HOMELAB_APPS__", string(appsJSON), 1)
	if _, err := c.Docker.ExecWithStdin(ctx, c.Container, bytes.NewReader([]byte(script)), "node"); err != nil {
		return fmt.Errorf("seeding Homarr default board: %w", err)
	}
	return nil
}

func seedApps(apps []App) []seededApp {
	result := make([]seededApp, 0, len(apps))
	seen := map[string]bool{}
	for _, app := range apps {
		if app.Name == "" || app.URL == "" {
			continue
		}
		slug := slugify(app.Name)
		if slug == "" || seen[slug] {
			continue
		}
		seen[slug] = true
		result = append(result, seededApp{
			ID:      "homelab_app_" + slug,
			ItemID:  "homelab_item_" + slug,
			Name:    app.Name,
			Href:    app.URL,
			IconURL: iconURL(app),
		})
	}
	return result
}

func iconURL(app App) string {
	if icon, ok := dashboardIconSlugs[app.Name]; ok {
		return "https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/svg/" + icon + ".svg"
	}
	return "https://www.google.com/s2/favicons?domain_url=" + app.URL + "&sz=64"
}

func slugify(value string) string {
	slug := strings.ToLower(value)
	slug = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(slug, "_")
	return strings.Trim(slug, "_")
}
