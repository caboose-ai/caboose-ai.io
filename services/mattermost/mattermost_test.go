package mattermost

import (
	"context"
	"strings"
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/docker"
	"github.com/caboose-ai/caboose-ai.io/internal/runner"
	"github.com/caboose-ai/caboose-ai.io/internal/secrets"
	"github.com/caboose-ai/caboose-ai.io/internal/service"
)

type memorySecrets struct {
	values map[string]string
}

func (m *memorySecrets) Get(_ context.Context, key string) (string, error) {
	return m.values[key], nil
}

func (m *memorySecrets) Put(_ context.Context, key, value string) error {
	m.values[key] = value
	return nil
}

func (m *memorySecrets) Generate(_ context.Context, key string, length int, _ secrets.GenerateOpts) (string, error) {
	m.values[key] = strings.Repeat("x", length)
	return m.values[key], nil
}

func (m *memorySecrets) Delete(_ context.Context, key string) error {
	delete(m.values, key)
	return nil
}

func (m *memorySecrets) EnsureVault(context.Context) error { return nil }

func TestConfigureCreatesAdminTeamAndMembership(t *testing.T) {
	r := runner.NewMockRunner()
	r.On("docker exec mattermost true", nil, nil)
	r.On("docker exec mattermost /mattermost/bin/mmctl --local user search", []byte(`[]`), nil)
	r.On("docker exec mattermost /mattermost/bin/mmctl --local user create", []byte("Created user auth-admin"), nil)
	r.On("docker exec mattermost /mattermost/bin/mmctl --local team create", []byte("New team caboose successfully created"), nil)
	r.On("docker exec mattermost /mattermost/bin/mmctl --local team users add", nil, nil)

	cfg := New(docker.NewExecClient(r), &memorySecrets{values: map[string]string{
		"MATTERMOST_ADMIN_PASSWORD": "SecretPass1!",
	}}, "mattermost", "admin@example.com")

	result, err := cfg.Configure(context.Background(), service.ConfigureOpts{})
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if result.Status != service.StatusCreated {
		t.Fatalf("status = %s, want %s", result.Status, service.StatusCreated)
	}
	if len(r.Calls) < 5 {
		t.Fatalf("calls = %d, want setup commands", len(r.Calls))
	}
}
