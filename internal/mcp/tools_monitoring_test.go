package mcp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/health"
)

type mockChecker struct {
	name string
	err  error
}

func (m *mockChecker) Name() string                     { return m.name }
func (m *mockChecker) Check(_ context.Context) error { return m.err }

func TestHandleHealthCheck(t *testing.T) {
	s := &Server{
		checkers: []health.Checker{
			&mockChecker{name: "Forgejo", err: nil},
			&mockChecker{name: "Grafana", err: fmt.Errorf("connection refused")},
		},
	}

	result, _, err := s.handleHealthCheck(context.Background(), nil, healthCheckInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := extractText(result)
	if !strings.Contains(text, "Forgejo") || !strings.Contains(text, "Grafana") {
		t.Errorf("expected both services in output: %q", text)
	}
	if !strings.Contains(text, `"healthy": true`) {
		t.Errorf("expected Forgejo to be healthy: %q", text)
	}
	if !strings.Contains(text, "connection refused") {
		t.Errorf("expected Grafana error: %q", text)
	}
}

func TestHandleHealthCheck_Filtered(t *testing.T) {
	s := &Server{
		checkers: []health.Checker{
			&mockChecker{name: "Forgejo", err: nil},
			&mockChecker{name: "Grafana", err: nil},
		},
	}

	result, _, err := s.handleHealthCheck(context.Background(), nil, healthCheckInput{Services: []string{"Forgejo"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := extractText(result)
	if !strings.Contains(text, "Forgejo") {
		t.Errorf("expected Forgejo in output: %q", text)
	}
	if strings.Contains(text, "Grafana") {
		t.Errorf("should not contain Grafana: %q", text)
	}
}

func TestHandleHealthCheck_NoCheckers(t *testing.T) {
	s := &Server{}

	_, _, err := s.handleHealthCheck(context.Background(), nil, healthCheckInput{})
	if err == nil {
		t.Fatal("expected error when no checkers configured")
	}
}

func TestHandleHealthCheckAll(t *testing.T) {
	s := &Server{
		checkers: []health.Checker{
			&mockChecker{name: "Authentik", err: nil},
			&mockChecker{name: "Forgejo", err: nil},
		},
	}

	result, _, err := s.handleHealthCheckAll(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := extractText(result)
	if !strings.Contains(text, "Authentik") || !strings.Contains(text, "Forgejo") {
		t.Errorf("expected all services: %q", text)
	}
}

func TestHandlePrometheusQuery(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("query")
		fmt.Fprintf(w, `{"status":"success","data":{"result":[{"metric":{},"value":[1,"%.2f"]}]},"query":%q}`, 42.0, q)
	}))
	defer ts.Close()

	s := &Server{http: ts.Client()}

	input := prometheusQueryInput{Query: "up"}
	result, _, err := s.handlePrometheusQuery(context.Background(), nil, input)
	if err != nil {
		// Prometheus may not be reachable in test, but handler should not panic
		if !strings.Contains(err.Error(), "prometheus_query") {
			t.Fatalf("unexpected error format: %v", err)
		}
		return
	}
	text := extractText(result)
	if !strings.Contains(text, "success") {
		t.Errorf("expected success response: %q", text)
	}
}

func TestHandlePrometheusQuery_Empty(t *testing.T) {
	s := &Server{}
	_, _, err := s.handlePrometheusQuery(context.Background(), nil, prometheusQueryInput{})
	if err == nil {
		t.Fatal("expected error for empty query")
	}
}

func TestHandleContainerStats(t *testing.T) {
	r := &mockRunner{output: []byte(`{"Name":"forgejo","CPUPerc":"0.5%","MemUsage":"100MiB"}`)}
	s := &Server{runner: r}

	result, _, err := s.handleContainerStats(context.Background(), nil, containerStatsInput{Service: "forgejo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := extractText(result)
	if !strings.Contains(text, "forgejo") {
		t.Errorf("expected container name in output: %q", text)
	}
	if !strings.Contains(strings.Join(r.lastArgs, " "), "forgejo") {
		t.Errorf("expected service name in docker args: %v", r.lastArgs)
	}
}
