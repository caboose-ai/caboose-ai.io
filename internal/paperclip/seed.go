package paperclip

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const SoftwareShopMission = "Operate, improve, deploy, monitor, and secure caboose-ai.io and its homelab services as a high-skill software development shop."

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func NewClient(baseURL, apiKey string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey, http: httpClient}
}

type SeedPlan struct {
	Company   CompanySeed
	Goal      GoalSeed
	Projects  []ProjectSeed
	Agents    []AgentSeed
	Routines  []RoutineSeed
	Authority AuthoritySeed
}

type CompanySeed struct {
	Name               string `json:"name"`
	Description        string `json:"description"`
	BudgetMonthlyCents int    `json:"budgetMonthlyCents,omitempty"`
}

type GoalSeed struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Level       string `json:"level"`
	Status      string `json:"status"`
}

type ProjectSeed struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Status      string         `json:"status"`
	Workspace   map[string]any `json:"workspace,omitempty"`
}

type AgentSeed struct {
	Name               string         `json:"name"`
	Role               string         `json:"role"`
	Title              string         `json:"title"`
	ReportsTo          string         `json:"reportsTo,omitempty"`
	Capabilities       string         `json:"capabilities"`
	AdapterType        string         `json:"adapterType"`
	AdapterConfig      map[string]any `json:"adapterConfig"`
	BudgetMonthlyCents int            `json:"budgetMonthlyCents"`
}

type RoutineSeed struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    string `json:"priority,omitempty"`
}

type AuthoritySeed struct {
	Policy string `json:"policy"`
}

type InternalDeliveryConfig struct {
	ForgejoURL            string
	ForgejoRepoURL        string
	ForgejoRemote         string
	BranchPrefix          string
	ReviewSurface         string
	WoodpeckerURL         string
	WoodpeckerPipeline    string
	PortainerURL          string
	RuntimeMutationPolicy string
}

type SeedReport struct {
	CompanyID string
	Created   map[string]int
	Existing  map[string]int
}

type apiEntity struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Title string `json:"title"`
}

func DefaultInternalDeliveryConfig(domain string) InternalDeliveryConfig {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		domain = "caboose-ai.io"
	}
	return InternalDeliveryConfig{
		ForgejoURL:            "https://git." + domain,
		ForgejoRepoURL:        "https://git." + domain + "/caboose-ai/caboose-ai.io.git",
		ForgejoRemote:         "forgejo",
		BranchPrefix:          "paperclip/",
		ReviewSurface:         "forgejo",
		WoodpeckerURL:         "https://ci." + domain,
		WoodpeckerPipeline:    ".woodpecker.yml",
		PortainerURL:          "https://docker." + domain,
		RuntimeMutationPolicy: "human_approval_required",
	}
}

func SoftwareShopPlan(repo string) SeedPlan {
	return SoftwareShopPlanWithDelivery(repo, DefaultInternalDeliveryConfig("caboose-ai.io"))
}

func SoftwareShopPlanWithDelivery(repo string, delivery InternalDeliveryConfig) SeedPlan {
	delivery = delivery.withDefaults()
	contextDoc := GeneratedContextDocumentWithDelivery(repo, delivery)
	return SeedPlan{
		Company: CompanySeed{
			Name:               "Caboose AI Software Shop",
			Description:        SoftwareShopMission,
			BudgetMonthlyCents: 10000,
		},
		Goal: GoalSeed{
			Title:       SoftwareShopMission,
			Description: contextDoc,
			Level:       "company",
			Status:      "active",
		},
		Projects: []ProjectSeed{
			projectWithDelivery("Homelab Core", "Installer, reset flows, compose, Caddy, and Authentik bootstrap.", repo, delivery),
			projectWithDelivery("SSO and Identity", "OAuth/OIDC, Authentik proxy apps, recovery flows, and smoke tests.", repo, delivery),
			projectWithDelivery("Observability", "Prometheus, Loki, Grafana, health checks, dashboards, and log review.", repo, delivery),
			projectWithDelivery("Service Workspaces", "Per-service manifests, configuration, docs, and service lifecycle boundaries.", repo, delivery),
			projectWithDelivery("Delivery", "Forgejo, Woodpecker, PRs, release verification, and deployment evidence.", repo, delivery),
			projectWithDelivery("Agent Control Plan", "plan-only v1 intake, approval, execution evidence, and follow-up loop for agent work. Use Forgejo/Gitea, Woodpecker, and Portainer as self-hosted delivery/control surfaces; Portainer and Docker mutations remain approval-gated.", repo, delivery),
		},
		Agents: []AgentSeed{
			agentWithDelivery("BoardOwner", "owner", "Board/Owner", "", "Human operator with final approval authority.", repo, 0, delivery),
			agentWithDelivery("CEO/PM", "ceo", "CEO/PM", "BoardOwner", "Triages goals, breaks down work, and requests approvals.", repo, 2000, delivery),
			agentWithDelivery("Architect", "architect", "Architect", "CEO/PM", "Designs repo and service changes, migration plans, and system boundaries.", repo, 1500, delivery),
			agentWithDelivery("BackendEngineer", "engineer", "Backend Engineer", "Architect", "Go installer, MCP, config, and service integration.", repo, 1500, delivery),
			agentWithDelivery("DevOpsSRE", "sre", "DevOps/SRE", "Architect", "Compose, Caddy, Authentik, Docker, metrics, logs, and deployment.", repo, 1500, delivery),
			agentWithDelivery("QAEngineer", "qa", "QA Engineer", "CEO/PM", "Unit tests, smoke tests, Selenium/Rod browser checks, and regression evidence.", repo, 1000, delivery),
			agentWithDelivery("SecurityEngineer", "security", "Security Engineer", "Architect", "Secret handling, Authentik/SSO, Semgrep, and dependency risk.", repo, 1000, delivery),
			agentWithDelivery("UIUXEngineer", "designer", "UI/UX Engineer", "CEO/PM", "Paperclip, Homarr, and Grafana-facing usability and visual review.", repo, 750, delivery),
			agentWithDelivery("RDEngineer", "researcher", "R&D Engineer", "CEO/PM", "Evaluates services, agent runtimes, and automation ideas.", repo, 750, delivery),
		},
		Routines: []RoutineSeed{
			{Title: "Daily service health and log review", Description: "Check service status, logs, and dashboards; file issues for drift.", Priority: "medium"},
			{Title: "Weekly SSO smoke-test report", Description: "Run quick SSO coverage and summarize Authentik/provider drift.", Priority: "medium"},
			{Title: "Weekly dependency/security review", Description: "Review dependency, Semgrep, and secret-handling risk.", Priority: "medium"},
			{Title: "Post-PR verification checklist", Description: "Verify tests, docs, smoke evidence, and approval boundaries after PRs.", Priority: "medium"},
			{Title: "Incident triage when health checks fail", Description: "Open a triage issue when health checks fail and propose non-destructive next steps.", Priority: "high"},
			{Title: "Agent task intake triage", Description: "Classify new agent-control tasks, capture affected services, budget, authority needs, and plan-only next steps.", Priority: "high"},
			{Title: "Agent plan approval gate", Description: "Review agent plans before any writes, infra mutation, deploy, Docker, Portainer, secret, or token-spending action.", Priority: "high"},
			{Title: "Agent execution evidence review", Description: "Check PR links, command output, smoke results, Woodpecker status, and rollback notes before accepting execution.", Priority: "medium"},
			{Title: "Agent follow-up task review", Description: "Convert unresolved findings into follow-up Paperclip tasks with owners, service scope, and approval requirements.", Priority: "medium"},
		},
		Authority: AuthoritySeed{Policy: "Default mode is plan-only: agents may inspect, branch, test, commit, open PRs, query monitoring, and propose deploy actions. Direct infra execution remains approval-gated. Docker and Portainer mutations, installer, reset, production deploy, secret, firewall, destructive docker commands, and other destructive commands require explicit human approval. Recurring jobs must stay within budget and write audit trails."},
	}
}

func GeneratedContextDocument(repo string) string {
	return GeneratedContextDocumentWithDelivery(repo, DefaultInternalDeliveryConfig("caboose-ai.io"))
}

func GeneratedContextDocumentWithDelivery(repo string, delivery InternalDeliveryConfig) string {
	delivery = delivery.withDefaults()
	return fmt.Sprintf("Repo: %s\nKey context: README.md, CLAUDE.md, service manifests, MCP tools, mise tasks, smoke tests, deployment rules, and Authentik SSO contracts. Default mode is plan-only. Use Forgejo/Gitea, Woodpecker, and Portainer as self-hosted delivery/control surfaces, but direct infra execution remains approval-gated. For implementation, push Paperclip work to Forgejo remote %s using %s branches, open Forgejo pull requests at %s, attach Woodpecker evidence from %s, and Portainer at %s is inspection-only unless a human explicitly approves a runtime mutation. Never deploy, reset, mutate secrets, change firewall rules, or run Portainer/Docker mutation commands without explicit human approval.", repo, delivery.ForgejoRemote, delivery.BranchPrefix, delivery.ForgejoURL, delivery.WoodpeckerURL, delivery.PortainerURL)
}

func project(name, description, repo string) ProjectSeed {
	return projectWithDelivery(name, description, repo, DefaultInternalDeliveryConfig("caboose-ai.io"))
}

func projectWithDelivery(name, description, repo string, delivery InternalDeliveryConfig) ProjectSeed {
	delivery = delivery.withDefaults()
	workspace := delivery.workspace(repo)
	return ProjectSeed{
		Name:        name,
		Description: description,
		Status:      "planned",
		Workspace:   workspace,
	}
}

func agent(name, role, title, reportsTo, capabilities, repo string, budget int) AgentSeed {
	return agentWithDelivery(name, role, title, reportsTo, capabilities, repo, budget, DefaultInternalDeliveryConfig("caboose-ai.io"))
}

func agentWithDelivery(name, role, title, reportsTo, capabilities, repo string, budget int, delivery InternalDeliveryConfig) AgentSeed {
	delivery = delivery.withDefaults()
	return AgentSeed{
		Name:         name,
		Role:         paperclipRole(role),
		Title:        title,
		ReportsTo:    "",
		Capabilities: capabilities,
		AdapterType:  "codex_local",
		AdapterConfig: map[string]any{
			"cwd":          repo,
			"approvalMode": "gated",
			"instructions": GeneratedContextDocumentWithDelivery(repo, delivery),
			"delivery":     delivery.workspace(repo),
		},
		BudgetMonthlyCents: budget,
	}
}

func (d InternalDeliveryConfig) withDefaults() InternalDeliveryConfig {
	defaults := DefaultInternalDeliveryConfig("caboose-ai.io")
	if d.ForgejoURL == "" {
		d.ForgejoURL = defaults.ForgejoURL
	}
	if d.ForgejoRepoURL == "" {
		d.ForgejoRepoURL = defaults.ForgejoRepoURL
	}
	if d.ForgejoRemote == "" {
		d.ForgejoRemote = defaults.ForgejoRemote
	}
	if d.BranchPrefix == "" {
		d.BranchPrefix = defaults.BranchPrefix
	}
	if d.ReviewSurface == "" {
		d.ReviewSurface = defaults.ReviewSurface
	}
	if d.WoodpeckerURL == "" {
		d.WoodpeckerURL = defaults.WoodpeckerURL
	}
	if d.WoodpeckerPipeline == "" {
		d.WoodpeckerPipeline = defaults.WoodpeckerPipeline
	}
	if d.PortainerURL == "" {
		d.PortainerURL = defaults.PortainerURL
	}
	if d.RuntimeMutationPolicy == "" {
		d.RuntimeMutationPolicy = defaults.RuntimeMutationPolicy
	}
	return d
}

func (d InternalDeliveryConfig) workspace(repo string) map[string]any {
	return map[string]any{
		"name":                  "caboose-ai.io",
		"cwd":                   repo,
		"repoRef":               "main",
		"defaultRef":            "main",
		"isPrimary":             true,
		"sourceType":            "local_path",
		"reviewSurface":         d.ReviewSurface,
		"repoUrl":               d.ForgejoRepoURL,
		"remoteName":            d.ForgejoRemote,
		"branchPrefix":          d.BranchPrefix,
		"ciProvider":            "woodpecker",
		"ciUrl":                 d.WoodpeckerURL,
		"ciPipeline":            d.WoodpeckerPipeline,
		"runtimeSurface":        "portainer",
		"runtimeInspectionUrl":  d.PortainerURL,
		"runtimeMutationPolicy": d.RuntimeMutationPolicy,
	}
}

func paperclipRole(role string) string {
	switch role {
	case "owner":
		return "pm"
	case "architect":
		return "cto"
	case "sre":
		return "devops"
	default:
		return role
	}
}

func SeedSoftwareShop(ctx context.Context, client *Client, repo string) (*SeedReport, error) {
	return SeedSoftwareShopWithDelivery(ctx, client, repo, DefaultInternalDeliveryConfig("caboose-ai.io"))
}

func SeedSoftwareShopWithDelivery(ctx context.Context, client *Client, repo string, delivery InternalDeliveryConfig) (*SeedReport, error) {
	plan := SoftwareShopPlanWithDelivery(repo, delivery)
	report := &SeedReport{Created: map[string]int{}, Existing: map[string]int{}}

	company, created, err := client.ensureCompany(ctx, plan.Company)
	if err != nil {
		return nil, err
	}
	report.CompanyID = company.ID
	count(report, "companies", created)

	if _, created, err := client.ensureByTitle(ctx, fmt.Sprintf("/api/companies/%s/goals", company.ID), plan.Goal.Title, plan.Goal); err != nil {
		return nil, err
	} else {
		count(report, "goals", created)
	}

	for _, p := range plan.Projects {
		if _, created, err := client.ensureByName(ctx, fmt.Sprintf("/api/companies/%s/projects", company.ID), p.Name, p); err != nil {
			return nil, err
		} else {
			count(report, "projects", created)
		}
	}
	for _, a := range plan.Agents {
		if _, created, err := client.ensureByName(ctx, fmt.Sprintf("/api/companies/%s/agents", company.ID), a.Name, a); err != nil {
			return nil, err
		} else {
			count(report, "agents", created)
		}
	}
	for _, r := range plan.Routines {
		if _, created, err := client.ensureByTitle(ctx, fmt.Sprintf("/api/companies/%s/routines", company.ID), r.Title, r); err != nil {
			return nil, err
		} else {
			count(report, "routines", created)
		}
	}
	return report, nil
}

func count(report *SeedReport, key string, created bool) {
	if created {
		report.Created[key]++
	} else {
		report.Existing[key]++
	}
}

func (c *Client) ensureCompany(ctx context.Context, seed CompanySeed) (apiEntity, bool, error) {
	return c.ensureByName(ctx, "/api/companies", seed.Name, seed)
}

func (c *Client) ensureByName(ctx context.Context, path, name string, payload any) (apiEntity, bool, error) {
	return c.ensure(ctx, path, func(e apiEntity) bool { return e.Name == name }, payload)
}

func (c *Client) ensureByTitle(ctx context.Context, path, title string, payload any) (apiEntity, bool, error) {
	return c.ensure(ctx, path, func(e apiEntity) bool { return e.Title == title }, payload)
}

func (c *Client) ensure(ctx context.Context, path string, match func(apiEntity) bool, payload any) (apiEntity, bool, error) {
	var existing []apiEntity
	if err := c.do(ctx, http.MethodGet, path, nil, &existing); err != nil {
		return apiEntity{}, false, err
	}
	for _, entity := range existing {
		if match(entity) {
			return entity, false, nil
		}
	}
	var created apiEntity
	if err := c.do(ctx, http.MethodPost, path, payload, &created); err != nil {
		return apiEntity{}, false, err
	}
	return created, true, nil
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var apiErr struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&apiErr)
		if apiErr.Error == "" {
			apiErr.Error = resp.Status
		}
		return fmt.Errorf("%s %s: %s", method, path, apiErr.Error)
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode %s %s: %w", method, path, err)
		}
	}
	return nil
}
