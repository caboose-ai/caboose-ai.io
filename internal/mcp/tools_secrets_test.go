package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/secrets"
)

type mockSecretStore struct {
	data         map[string]string
	lastGenerate struct {
		key    string
		length int
		opts   secrets.GenerateOpts
	}
}

func (m *mockSecretStore) Get(_ context.Context, key string) (string, error) {
	return m.data[key], nil
}
func (m *mockSecretStore) Put(_ context.Context, key, value string) error {
	m.data[key] = value
	return nil
}
func (m *mockSecretStore) Generate(_ context.Context, key string, length int, opts secrets.GenerateOpts) (string, error) {
	m.lastGenerate.key = key
	m.lastGenerate.length = length
	m.lastGenerate.opts = opts
	val := "generated-" + key
	m.data[key] = val
	return val, nil
}
func (m *mockSecretStore) Delete(_ context.Context, key string) error {
	delete(m.data, key)
	return nil
}
func (m *mockSecretStore) EnsureVault(_ context.Context) error { return nil }

func TestHandleSecretsGet(t *testing.T) {
	s := &Server{secrets: &mockSecretStore{data: map[string]string{"MY_KEY": "my-value"}}}

	result, _, err := s.handleSecretsGet(context.Background(), nil, secretsGetInput{Key: "MY_KEY"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := extractText(result)
	if text != "my-value" {
		t.Errorf("expected 'my-value', got %q", text)
	}
}

func TestHandleSecretsGet_NotFound(t *testing.T) {
	s := &Server{secrets: &mockSecretStore{data: map[string]string{}}}

	result, _, err := s.handleSecretsGet(context.Background(), nil, secretsGetInput{Key: "MISSING"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := extractText(result)
	if !strings.Contains(text, "not found") {
		t.Errorf("expected 'not found' message, got %q", text)
	}
}

func TestHandleSecretsGet_EmptyKey(t *testing.T) {
	s := &Server{secrets: &mockSecretStore{data: map[string]string{}}}
	_, _, err := s.handleSecretsGet(context.Background(), nil, secretsGetInput{})
	if err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestHandleSecretsPut(t *testing.T) {
	store := &mockSecretStore{data: map[string]string{}}
	s := &Server{secrets: store}

	result, _, err := s.handleSecretsPut(context.Background(), nil, secretsPutInput{Key: "NEW_KEY", Value: "new-val"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := extractText(result)
	if !strings.Contains(text, "stored successfully") {
		t.Errorf("unexpected result: %q", text)
	}
	if store.data["NEW_KEY"] != "new-val" {
		t.Errorf("secret not stored")
	}
}

func TestHandleSecretsGenerate(t *testing.T) {
	store := &mockSecretStore{data: map[string]string{}}
	s := &Server{secrets: store}

	result, _, err := s.handleSecretsGenerate(context.Background(), nil, secretsGenerateInput{Key: "GEN_KEY"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := extractText(result)
	if !strings.Contains(text, "generated-GEN_KEY") {
		t.Errorf("unexpected result: %q", text)
	}
	if store.lastGenerate.length != 32 {
		t.Errorf("expected default length 32, got %d", store.lastGenerate.length)
	}
	if store.lastGenerate.opts.Recipe != "letters,digits,32" {
		t.Errorf("expected default recipe to include default length, got %q", store.lastGenerate.opts.Recipe)
	}
}

func TestHandleSecretsGenerate_CustomLengthUsesMatchingDefaultRecipe(t *testing.T) {
	store := &mockSecretStore{data: map[string]string{}}
	s := &Server{secrets: store}

	_, _, err := s.handleSecretsGenerate(context.Background(), nil, secretsGenerateInput{
		Key:    "GEN_KEY",
		Length: 64,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.lastGenerate.length != 64 {
		t.Errorf("expected length 64, got %d", store.lastGenerate.length)
	}
	if store.lastGenerate.opts.Recipe != "letters,digits,64" {
		t.Errorf("expected generated default recipe to honor requested length, got %q", store.lastGenerate.opts.Recipe)
	}
}

func TestHandleSecretsList(t *testing.T) {
	s := &Server{secrets: &mockSecretStore{data: map[string]string{}}}

	result, _, err := s.handleSecretsList(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := extractText(result)
	if !strings.Contains(text, "AUTHENTIK_SECRET_KEY") {
		t.Errorf("expected bootstrap secrets in output, got %q", text)
	}
}
