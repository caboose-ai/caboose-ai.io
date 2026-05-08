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
			_ = json.NewEncoder(w).Encode([]map[string]any{{"id": "routine-1", "name": "Daily service health and log review"}})
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
