package install

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
)

type paperclipRunner struct {
	command string
	args    []string
}

func (r *paperclipRunner) Run(_ context.Context, command string, args ...string) ([]byte, error) {
	r.command = command
	r.args = append([]string(nil), args...)
	return nil, nil
}

func (r *paperclipRunner) RunWithStdin(context.Context, io.Reader, string, ...string) ([]byte, error) {
	return nil, nil
}

func TestBootstrapPaperclipStartsProfileAndSeedsSoftwareShop(t *testing.T) {
	var companyCreated bool
	var createdGoals int
	var createdProjects int
	var createdAgents int
	var createdRoutines int

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/api/companies", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode([]map[string]any{})
		case http.MethodPost:
			companyCreated = true
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "company-1", "name": "Caboose AI Software Shop"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	mux.HandleFunc("/api/companies/company-1/goals", collectionHandler(t, &createdGoals, "goal"))
	mux.HandleFunc("/api/companies/company-1/projects", collectionHandler(t, &createdProjects, "project"))
	mux.HandleFunc("/api/companies/company-1/agents", collectionHandler(t, &createdAgents, "agent"))
	mux.HandleFunc("/api/companies/company-1/routines", collectionHandler(t, &createdRoutines, "routine"))
	server := httptest.NewServer(mux)
	defer server.Close()

	runner := &paperclipRunner{}
	cfg := config.DefaultConfig()
	cfg.Domain = "example.com"
	cfg.ComposeDir = "/opt/homelab"
	inst := New(cfg, &mockSecrets{store: map[string]string{}}, runner, http.DefaultClient)

	report, err := inst.BootstrapPaperclip(context.Background(), PaperclipBootstrapOptions{
		APIBase: server.URL,
		Repo:    "/repo",
	})
	if err != nil {
		t.Fatalf("BootstrapPaperclip: %v", err)
	}

	wantArgs := []string{
		"compose",
		"-f", "/opt/homelab/docker-compose.yml",
		"--profile", "paperclip",
		"up",
		"-d",
		"paperclip",
		"paperclip-db",
	}
	if runner.command != "docker" {
		t.Fatalf("command = %q, want docker", runner.command)
	}
	if !reflect.DeepEqual(runner.args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", runner.args, wantArgs)
	}
	if !companyCreated {
		t.Fatal("company was not created")
	}
	if report.CompanyID != "company-1" {
		t.Fatalf("CompanyID = %q, want company-1", report.CompanyID)
	}
	if createdGoals != 1 || createdProjects != 6 || createdAgents != 9 || createdRoutines != 9 {
		t.Fatalf("created goals/projects/agents/routines = %d/%d/%d/%d, want 1/6/9/9", createdGoals, createdProjects, createdAgents, createdRoutines)
	}
}

func collectionHandler(t *testing.T, created *int, idPrefix string) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode([]map[string]any{})
		case http.MethodPost:
			(*created)++
			_ = json.NewEncoder(w).Encode(map[string]any{"id": idPrefix})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}
}
