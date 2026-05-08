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
	Name        string `json:"name"`
	Description string `json:"description"`
	Schedule    string `json:"schedule"`
	BudgetCents int    `json:"budgetCents"`
}

type AuthoritySeed struct {
	Policy string `json:"policy"`
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

func SoftwareShopPlan(repo string) SeedPlan {
	contextDoc := GeneratedContextDocument(repo)
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
			project("Homelab Core", "Installer, reset flows, compose, Caddy, and Authentik bootstrap.", repo),
			project("SSO and Identity", "OAuth/OIDC, Authentik proxy apps, recovery flows, and smoke tests.", repo),
			project("Observability", "Prometheus, Loki, Grafana, health checks, dashboards, and log review.", repo),
			project("Service Workspaces", "Per-service manifests, configuration, docs, and service lifecycle boundaries.", repo),
			project("Delivery", "Forgejo, Woodpecker, PRs, release verification, and deployment evidence.", repo),
		},
		Agents: []AgentSeed{
			agent("BoardOwner", "owner", "Board/Owner", "", "Human operator with final approval authority.", repo, 0),
			agent("CEO/PM", "ceo", "CEO/PM", "BoardOwner", "Triages goals, breaks down work, and requests approvals.", repo, 2000),
			agent("Architect", "architect", "Architect", "CEO/PM", "Designs repo and service changes, migration plans, and system boundaries.", repo, 1500),
			agent("BackendEngineer", "engineer", "Backend Engineer", "Architect", "Go installer, MCP, config, and service integration.", repo, 1500),
			agent("DevOpsSRE", "sre", "DevOps/SRE", "Architect", "Compose, Caddy, Authentik, Docker, metrics, logs, and deployment.", repo, 1500),
			agent("QAEngineer", "qa", "QA Engineer", "CEO/PM", "Unit tests, smoke tests, Selenium/Rod browser checks, and regression evidence.", repo, 1000),
			agent("SecurityEngineer", "security", "Security Engineer", "Architect", "Secret handling, Authentik/SSO, Semgrep, and dependency risk.", repo, 1000),
			agent("UIUXEngineer", "designer", "UI/UX Engineer", "CEO/PM", "Paperclip, Homarr, and Grafana-facing usability and visual review.", repo, 750),
			agent("RDEngineer", "researcher", "R&D Engineer", "CEO/PM", "Evaluates services, agent runtimes, and automation ideas.", repo, 750),
		},
		Routines: []RoutineSeed{
			{Name: "Daily service health and log review", Description: "Check service status, logs, and dashboards; file issues for drift.", Schedule: "daily", BudgetCents: 150},
			{Name: "Weekly SSO smoke-test report", Description: "Run quick SSO coverage and summarize Authentik/provider drift.", Schedule: "weekly", BudgetCents: 250},
			{Name: "Weekly dependency/security review", Description: "Review dependency, Semgrep, and secret-handling risk.", Schedule: "weekly", BudgetCents: 250},
			{Name: "Post-PR verification checklist", Description: "Verify tests, docs, smoke evidence, and approval boundaries after PRs.", Schedule: "event:pull_request", BudgetCents: 200},
			{Name: "Incident triage when health checks fail", Description: "Open a triage issue when health checks fail and propose non-destructive next steps.", Schedule: "event:health_failure", BudgetCents: 200},
		},
		Authority: AuthoritySeed{Policy: "Agents may inspect, branch, test, commit, open PRs, query monitoring, and propose deploy actions. docker, installer, reset, production deploy, secret, firewall, and destructive commands require explicit human approval. Recurring jobs must stay within budget and write audit trails."},
	}
}

func GeneratedContextDocument(repo string) string {
	return fmt.Sprintf("Repo: %s\nKey context: README.md, CLAUDE.md, service manifests, MCP tools, mise tasks, smoke tests, deployment rules, and Authentik SSO contracts. Never deploy, reset, mutate secrets, change firewall rules, or run destructive Docker commands without explicit human approval.", repo)
}

func project(name, description, repo string) ProjectSeed {
	return ProjectSeed{
		Name:        name,
		Description: description,
		Status:      "planned",
		Workspace: map[string]any{
			"name":      "caboose-ai.io",
			"cwd":       repo,
			"repoRef":   "main",
			"isPrimary": true,
		},
	}
}

func agent(name, role, title, reportsTo, capabilities, repo string, budget int) AgentSeed {
	return AgentSeed{
		Name:         name,
		Role:         role,
		Title:        title,
		ReportsTo:    reportsTo,
		Capabilities: capabilities,
		AdapterType:  "codex_local",
		AdapterConfig: map[string]any{
			"cwd":          repo,
			"approvalMode": "gated",
			"instructions": GeneratedContextDocument(repo),
		},
		BudgetMonthlyCents: budget,
	}
}

func SeedSoftwareShop(ctx context.Context, client *Client, repo string) (*SeedReport, error) {
	plan := SoftwareShopPlan(repo)
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
		if _, created, err := client.ensureByName(ctx, fmt.Sprintf("/api/companies/%s/routines", company.ID), r.Name, r); err != nil {
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
