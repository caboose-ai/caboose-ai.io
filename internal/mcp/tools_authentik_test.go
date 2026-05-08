package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/service"
)

func TestHandleAuthentikListProviders_NoAK(t *testing.T) {
	s := &Server{ak: nil}
	_, _, err := s.handleAuthentikListProviders(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected error when Authentik not initialized")
	}
	if !strings.Contains(err.Error(), "homelab install") {
		t.Errorf("error should suggest running install, got %q", err.Error())
	}
}

func TestHandleAuthentikGetProvider_NoAK(t *testing.T) {
	s := &Server{ak: nil}
	_, _, err := s.handleAuthentikGetProvider(context.Background(), nil, authentikGetProviderInput{Name: "test"})
	if err == nil {
		t.Fatal("expected error when Authentik not initialized")
	}
}

func TestHandleAuthentikListSources_NoAK(t *testing.T) {
	s := &Server{ak: nil}
	_, _, err := s.handleAuthentikListSources(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected error when Authentik not initialized")
	}
}

func TestHandleAuthentikUpsertSource_NoAK(t *testing.T) {
	s := &Server{ak: nil}
	_, _, err := s.handleAuthentikUpsertSource(context.Background(), nil, authentikUpsertSourceInput{Slug: "github"})
	if err == nil {
		t.Fatal("expected error when Authentik not initialized")
	}
}

func TestHandleAuthentikFindUser_NoAK(t *testing.T) {
	s := &Server{ak: nil}
	_, _, err := s.handleAuthentikFindUser(context.Background(), nil, authentikFindUserInput{Username: "admin"})
	if err == nil {
		t.Fatal("expected error when Authentik not initialized")
	}
}

func TestHandleAuthentikRenameUser_NoAK(t *testing.T) {
	s := &Server{ak: nil}
	_, _, err := s.handleAuthentikRenameUser(context.Background(), nil, authentikRenameUserInput{CurrentUsername: "old", NewUsername: "new"})
	if err == nil {
		t.Fatal("expected error when Authentik not initialized")
	}
}

func TestHandleSSOConfigureService_NoAK(t *testing.T) {
	s := &Server{ak: nil}
	_, _, err := s.handleSSOConfigureService(context.Background(), nil, ssoConfigureServiceInput{Service: "forgejo"})
	if err == nil {
		t.Fatal("expected error when Authentik not initialized")
	}
}

type mockServiceConfigurator struct {
	slug   string
	name   string
	result *service.ConfigureResult
	err    error
}

func (m *mockServiceConfigurator) Name() string { return m.name }
func (m *mockServiceConfigurator) Slug() string { return m.slug }
func (m *mockServiceConfigurator) CheckConfigured(_ context.Context) (bool, error) {
	return m.result != nil, nil
}
func (m *mockServiceConfigurator) Configure(_ context.Context, _ service.ConfigureOpts) (*service.ConfigureResult, error) {
	return m.result, m.err
}

func TestHandleSSOConfigureService_ValidService(t *testing.T) {
	s := &Server{
		ak: dummyAK(),
		services: []service.ServiceConfigurator{
			&mockServiceConfigurator{
				slug: "forgejo",
				name: "Forgejo",
				result: &service.ConfigureResult{
					Status:  service.StatusCreated,
					Message: "OAuth provider created",
				},
			},
		},
	}

	result, _, err := s.handleSSOConfigureService(context.Background(), nil, ssoConfigureServiceInput{Service: "forgejo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := extractText(result)
	if !strings.Contains(text, "OAuth provider created") {
		t.Errorf("expected result message, got %q", text)
	}
}

func TestHandleSSOConfigureService_UnknownService(t *testing.T) {
	s := &Server{
		ak: dummyAK(),
		registry: service.NewRegistry(map[string]service.Manifest{
			"forgejo": {Slug: "forgejo", DisplayName: "Forgejo", Configurator: "forgejo"},
			"grafana": {Slug: "grafana", DisplayName: "Grafana", Configurator: "grafana"},
		}, []service.ServiceConfigurator{
			&mockServiceConfigurator{slug: "forgejo", name: "Forgejo"},
			&mockServiceConfigurator{slug: "grafana", name: "Grafana"},
		}),
	}

	_, _, err := s.handleSSOConfigureService(context.Background(), nil, ssoConfigureServiceInput{Service: "unknown"})
	if err == nil {
		t.Fatal("expected error for unknown service")
	}
	if !strings.Contains(err.Error(), "forgejo") || !strings.Contains(err.Error(), "grafana") {
		t.Errorf("error should list valid slugs, got %q", err.Error())
	}
}
