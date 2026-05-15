package paperclip

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSoftwareShopPlanRequiresApprovalForDangerousCommands(t *testing.T) {
	plan := SoftwareShopPlan("/repo")

	dangerous := []string{"docker", "deploy", "reset", "secret", "firewall", "destructive"}
	for _, word := range dangerous {
		if !strings.Contains(plan.Authority.Policy, word) {
			t.Fatalf("authority policy missing %q: %s", word, plan.Authority.Policy)
		}
	}
}

func TestSoftwareShopPlanSeedsAgentControlPlanProject(t *testing.T) {
	plan := SoftwareShopPlan("/repo")
	project := findProject(t, plan, "Agent Control Plan")

	wantDescription := []string{"plan-only", "Forgejo", "Gitea", "Woodpecker", "Portainer", "approval-gated"}
	for _, word := range wantDescription {
		if !strings.Contains(project.Description, word) {
			t.Fatalf("Agent Control Plan description missing %q: %s", word, project.Description)
		}
	}
	if project.Workspace["cwd"] != "/repo" {
		t.Fatalf("workspace cwd = %v, want /repo", project.Workspace["cwd"])
	}
	if project.Workspace["repoRef"] != "main" {
		t.Fatalf("workspace repoRef = %v, want main", project.Workspace["repoRef"])
	}
	if project.Workspace["isPrimary"] != true {
		t.Fatalf("workspace isPrimary = %v, want true", project.Workspace["isPrimary"])
	}
}

func TestSoftwareShopPlanSeedsInternalDeliveryWorkspace(t *testing.T) {
	plan := SoftwareShopPlan("/repo")
	project := findProject(t, plan, "Delivery")

	want := map[string]any{
		"sourceType":            "local_path",
		"reviewSurface":         "forgejo",
		"repoUrl":               "https://git.caboose-ai.io/caboose-ai/caboose-ai.io.git",
		"remoteName":            "forgejo",
		"branchPrefix":          "paperclip/",
		"ciProvider":            "woodpecker",
		"ciUrl":                 "https://ci.caboose-ai.io",
		"ciPipeline":            ".woodpecker.yml",
		"runtimeSurface":        "portainer",
		"runtimeInspectionUrl":  "https://docker.caboose-ai.io",
		"runtimeMutationPolicy": "human_approval_required",
	}
	for key, value := range want {
		if project.Workspace[key] != value {
			t.Fatalf("workspace[%q] = %v, want %v", key, project.Workspace[key], value)
		}
	}
}

func TestSoftwareShopPlanSupportsCustomInternalDelivery(t *testing.T) {
	delivery := DefaultInternalDeliveryConfig("example.test")
	delivery.ForgejoRepoURL = "https://git.example.test/team/repo.git"
	delivery.ForgejoRemote = "internal"
	delivery.BranchPrefix = "agent/"

	plan := SoftwareShopPlanWithDelivery("/repo", delivery)
	project := findProject(t, plan, "Delivery")

	if project.Workspace["repoUrl"] != "https://git.example.test/team/repo.git" {
		t.Fatalf("repoUrl = %v", project.Workspace["repoUrl"])
	}
	if project.Workspace["remoteName"] != "internal" {
		t.Fatalf("remoteName = %v", project.Workspace["remoteName"])
	}
	if project.Workspace["branchPrefix"] != "agent/" {
		t.Fatalf("branchPrefix = %v", project.Workspace["branchPrefix"])
	}
	if project.Workspace["ciUrl"] != "https://ci.example.test" {
		t.Fatalf("ciUrl = %v", project.Workspace["ciUrl"])
	}
	if project.Workspace["runtimeInspectionUrl"] != "https://docker.example.test" {
		t.Fatalf("runtimeInspectionUrl = %v", project.Workspace["runtimeInspectionUrl"])
	}
}

func TestSoftwareShopPlanInstructsAgentsToUseInternalDelivery(t *testing.T) {
	plan := SoftwareShopPlan("/repo")

	for _, agent := range plan.Agents {
		delivery, ok := agent.AdapterConfig["delivery"].(map[string]any)
		if !ok {
			t.Fatalf("agent %s missing delivery adapter config: %#v", agent.Name, agent.AdapterConfig)
		}
		if delivery["reviewSurface"] != "forgejo" {
			t.Fatalf("agent %s reviewSurface = %v, want forgejo", agent.Name, delivery["reviewSurface"])
		}
		if delivery["ciProvider"] != "woodpecker" {
			t.Fatalf("agent %s ciProvider = %v, want woodpecker", agent.Name, delivery["ciProvider"])
		}
		if delivery["runtimeMutationPolicy"] != "human_approval_required" {
			t.Fatalf("agent %s runtimeMutationPolicy = %v, want human_approval_required", agent.Name, delivery["runtimeMutationPolicy"])
		}
	}

	contextDoc := GeneratedContextDocument("/repo")
	wantContext := []string{
		"push Paperclip work to Forgejo remote forgejo",
		"open Forgejo pull requests",
		"attach Woodpecker evidence from https://ci.caboose-ai.io",
		"Portainer at https://docker.caboose-ai.io is inspection-only",
	}
	for _, want := range wantContext {
		if !strings.Contains(contextDoc, want) {
			t.Fatalf("generated context missing %q: %s", want, contextDoc)
		}
	}
}

func TestSoftwareShopPlanSeedsAgentControlRoutines(t *testing.T) {
	plan := SoftwareShopPlan("/repo")
	want := []string{
		"Agent task intake triage",
		"Agent plan approval gate",
		"Agent execution evidence review",
		"Agent follow-up task review",
	}

	for _, title := range want {
		if !hasRoutine(plan, title) {
			t.Fatalf("missing routine %q", title)
		}
	}
}

func TestSoftwareShopPlanDefaultsToPlanOnlyAuthority(t *testing.T) {
	plan := SoftwareShopPlan("/repo")

	want := []string{"plan-only", "approval-gated", "Portainer", "Docker"}
	for _, word := range want {
		if !strings.Contains(plan.Authority.Policy, word) {
			t.Fatalf("authority policy missing %q: %s", word, plan.Authority.Policy)
		}
	}

	contextDoc := GeneratedContextDocument("/repo")
	for _, word := range []string{"plan-only", "Forgejo", "Gitea", "Woodpecker", "Portainer", "approval-gated"} {
		if !strings.Contains(contextDoc, word) {
			t.Fatalf("generated context missing %q: %s", word, contextDoc)
		}
	}
}

func TestSeedCompanyIsIdempotent(t *testing.T) {
	var createdCompanies int
	var createdAgents int
	var createdProjects int
	var createdRoutines int

	mux := http.NewServeMux()
	mux.HandleFunc("/api/companies", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode([]map[string]any{{"id": "company-1", "name": "Caboose AI Software Shop"}})
		case http.MethodPost:
			createdCompanies++
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "company-1", "name": "Caboose AI Software Shop"})
		default:
			t.Fatalf("unexpected method %s %s", r.Method, r.URL.Path)
		}
	})
	mux.HandleFunc("/api/companies/company-1/goals", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{{"id": "goal-1", "title": SoftwareShopMission}})
	})
	mux.HandleFunc("/api/companies/company-1/projects", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode([]map[string]any{{"id": "project-1", "name": "Homelab Core"}})
		case http.MethodPost:
			createdProjects++
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "project-new"})
		}
	})
	mux.HandleFunc("/api/companies/company-1/agents", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode([]map[string]any{{"id": "agent-1", "name": "CEO/PM"}})
		case http.MethodPost:
			createdAgents++
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "agent-new"})
		}
	})
	mux.HandleFunc("/api/companies/company-1/routines", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode([]map[string]any{{"id": "routine-1", "title": "Daily service health and log review"}})
		case http.MethodPost:
			createdRoutines++
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "routine-new"})
		}
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewClient(srv.URL, "token", srv.Client())
	report, err := SeedSoftwareShop(context.Background(), client, "/repo")
	if err != nil {
		t.Fatalf("SeedSoftwareShop: %v", err)
	}

	if createdCompanies != 0 {
		t.Fatalf("createdCompanies = %d, want 0", createdCompanies)
	}
	if createdAgents == 0 || createdProjects == 0 || createdRoutines == 0 {
		t.Fatalf("expected missing seed records to be created: agents=%d projects=%d routines=%d", createdAgents, createdProjects, createdRoutines)
	}
	if report.CompanyID != "company-1" {
		t.Fatalf("CompanyID = %q", report.CompanyID)
	}
}

func findProject(t *testing.T, plan SeedPlan, name string) ProjectSeed {
	t.Helper()
	for _, project := range plan.Projects {
		if project.Name == name {
			return project
		}
	}
	t.Fatalf("missing project %q", name)
	return ProjectSeed{}
}

func hasRoutine(plan SeedPlan, title string) bool {
	for _, routine := range plan.Routines {
		if routine.Title == title {
			return true
		}
	}
	return false
}
