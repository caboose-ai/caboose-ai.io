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
		"sourceType": "local_path",
		"repoUrl":    "https://git.caboose-ai.io/caboose-ai/caboose-ai.io.git",
	}
	for key, value := range want {
		if project.Workspace[key] != value {
			t.Fatalf("workspace[%q] = %v, want %v", key, project.Workspace[key], value)
		}
	}
	delivery := workspaceDeliveryMetadata(t, project.Workspace)
	wantDelivery := map[string]any{
		"reviewSurface":         "forgejo",
		"remoteName":            "forgejo",
		"branchPrefix":          "paperclip/",
		"ciProvider":            "woodpecker",
		"ciUrl":                 "https://ci.caboose-ai.io",
		"ciPipeline":            ".woodpecker.yml",
		"runtimeSurface":        "portainer",
		"runtimeInspectionUrl":  "https://docker.caboose-ai.io",
		"runtimeMutationPolicy": "human_approval_required",
	}
	for key, value := range wantDelivery {
		if delivery[key] != value {
			t.Fatalf("workspace metadata delivery[%q] = %v, want %v", key, delivery[key], value)
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
	deliveryMetadata := workspaceDeliveryMetadata(t, project.Workspace)
	if deliveryMetadata["remoteName"] != "internal" {
		t.Fatalf("remoteName = %v", deliveryMetadata["remoteName"])
	}
	if deliveryMetadata["branchPrefix"] != "agent/" {
		t.Fatalf("branchPrefix = %v", deliveryMetadata["branchPrefix"])
	}
	if deliveryMetadata["ciUrl"] != "https://ci.example.test" {
		t.Fatalf("ciUrl = %v", deliveryMetadata["ciUrl"])
	}
	if deliveryMetadata["runtimeInspectionUrl"] != "https://docker.example.test" {
		t.Fatalf("runtimeInspectionUrl = %v", deliveryMetadata["runtimeInspectionUrl"])
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
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id":               "project-1",
				"name":             "Homelab Core",
				"primaryWorkspace": map[string]any{"id": "workspace-1", "name": "caboose-ai.io"},
			}})
		case http.MethodPost:
			createdProjects++
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "project-new"})
		}
	})
	mux.HandleFunc("/api/projects/project-1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("unexpected method %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "project-1", "name": "Homelab Core"})
	})
	mux.HandleFunc("/api/projects/project-1/workspaces/workspace-1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("unexpected method %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "workspace-1", "name": "caboose-ai.io"})
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
	mux.HandleFunc("/api/agents/agent-1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("unexpected method %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "agent-1", "name": "CEO/PM"})
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

func TestSeedCompanyUpdatesExistingDeliveryMetadata(t *testing.T) {
	var patchedProject map[string]any
	var patchedDelivery map[string]any
	var patchedAgent map[string]any

	mux := http.NewServeMux()
	mux.HandleFunc("/api/companies", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{{"id": "company-1", "name": "Caboose AI Software Shop"}})
	})
	mux.HandleFunc("/api/companies/company-1/goals", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode([]map[string]any{{"id": "goal-1", "title": SoftwareShopMission}})
		default:
			t.Fatalf("unexpected method %s %s", r.Method, r.URL.Path)
		}
	})
	mux.HandleFunc("/api/companies/company-1/projects", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id":               "delivery-1",
				"name":             "Delivery",
				"primaryWorkspace": map[string]any{"id": "workspace-1", "name": "caboose-ai.io"},
			}})
		case http.MethodPost:
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "project-new"})
		default:
			t.Fatalf("unexpected method %s %s", r.Method, r.URL.Path)
		}
	})
	mux.HandleFunc("/api/projects/delivery-1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("unexpected method %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&patchedProject); err != nil {
			t.Fatalf("decode project patch: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "delivery-1", "name": "Delivery"})
	})
	mux.HandleFunc("/api/projects/delivery-1/workspaces/workspace-1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("unexpected method %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&patchedDelivery); err != nil {
			t.Fatalf("decode workspace patch: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "workspace-1", "name": "caboose-ai.io"})
	})
	mux.HandleFunc("/api/companies/company-1/agents", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode([]map[string]any{{"id": "agent-1", "name": "CEO/PM"}})
		case http.MethodPost:
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "agent-new"})
		default:
			t.Fatalf("unexpected method %s %s", r.Method, r.URL.Path)
		}
	})
	mux.HandleFunc("/api/agents/agent-1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("unexpected method %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&patchedAgent); err != nil {
			t.Fatalf("decode agent patch: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "agent-1", "name": "CEO/PM"})
	})
	mux.HandleFunc("/api/companies/company-1/routines", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode([]map[string]any{})
		case http.MethodPost:
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "routine-new"})
		default:
			t.Fatalf("unexpected method %s %s", r.Method, r.URL.Path)
		}
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewClient(srv.URL, "token", srv.Client())
	if _, err := SeedSoftwareShop(context.Background(), client, "/repo"); err != nil {
		t.Fatalf("SeedSoftwareShop: %v", err)
	}

	if _, ok := patchedProject["workspace"]; ok {
		t.Fatalf("project patch should not send create-only workspace field: %#v", patchedProject)
	}
	if _, ok := patchedProject["status"]; ok {
		t.Fatalf("project patch should not reset existing workflow status: %#v", patchedProject)
	}
	metadata := workspaceDeliveryMetadata(t, patchedDelivery)
	if metadata["reviewSurface"] != "forgejo" || metadata["ciProvider"] != "woodpecker" {
		t.Fatalf("patched delivery workspace metadata = %#v", metadata)
	}
	adapterConfig, ok := patchedAgent["adapterConfig"].(map[string]any)
	if !ok {
		t.Fatalf("patched agent missing adapterConfig: %#v", patchedAgent)
	}
	delivery, ok := adapterConfig["delivery"].(map[string]any)
	if !ok {
		t.Fatalf("patched agent missing delivery config: %#v", adapterConfig)
	}
	if delivery["runtimeMutationPolicy"] != "human_approval_required" {
		t.Fatalf("patched agent delivery = %#v", delivery)
	}
}

func workspaceDeliveryMetadata(t *testing.T, workspace map[string]any) map[string]any {
	t.Helper()
	metadata, ok := workspace["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("workspace missing metadata: %#v", workspace)
	}
	delivery, ok := metadata["delivery"].(map[string]any)
	if !ok {
		t.Fatalf("workspace metadata missing delivery: %#v", metadata)
	}
	return delivery
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
