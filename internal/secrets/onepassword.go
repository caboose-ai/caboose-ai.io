package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/caboose-ai/caboose-ai.io/internal/runner"
)

type OnePasswordStore struct {
	Vault  string
	Runner runner.CommandRunner
	Env    *EnvFileStore
}

func NewOnePasswordStore(vault string, r runner.CommandRunner, envPath string) *OnePasswordStore {
	return &OnePasswordStore{
		Vault:  vault,
		Runner: r,
		Env:    NewEnvFileStore(envPath),
	}
}

func (s *OnePasswordStore) EnsureVault(ctx context.Context) error {
	_, err := s.Runner.Run(ctx, "op", "vault", "get", s.Vault, "--format=json")
	if err == nil {
		return nil
	}
	_, err = s.Runner.Run(ctx, "op", "vault", "create", s.Vault)
	if err != nil {
		return fmt.Errorf("creating 1Password vault %q: %w", s.Vault, err)
	}
	return nil
}

func (s *OnePasswordStore) Get(ctx context.Context, key string) (string, error) {
	out, err := s.Runner.Run(ctx, "op", "item", "get", key,
		"--vault", s.Vault,
		"--fields", "password",
		"--format=json",
	)
	if err != nil {
		return s.Env.Get(ctx, key)
	}

	var field struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(out, &field); err != nil {
		return s.Env.Get(ctx, key)
	}

	// Sync to .env so Docker Compose can read it.
	if field.Value != "" {
		s.Env.Put(ctx, key, field.Value)
	}
	return field.Value, nil
}

func (s *OnePasswordStore) Put(ctx context.Context, key, value string) error {
	if err := s.Env.Put(ctx, key, value); err != nil {
		return err
	}

	existing, _ := s.Runner.Run(ctx, "op", "item", "get", key,
		"--vault", s.Vault, "--format=json")

	if existing != nil {
		_, err := s.Runner.Run(ctx, "op", "item", "edit", key,
			"password="+value,
			"--vault", s.Vault,
		)
		return err
	}

	_, err := s.Runner.Run(ctx, "op", "item", "create",
		"--category=password",
		"--title="+key,
		"--vault="+s.Vault,
		"password="+value,
	)
	return err
}

func (s *OnePasswordStore) Generate(ctx context.Context, key string, length int, opts GenerateOpts) (string, error) {
	existing, err := s.Get(ctx, key)
	if err == nil && existing != "" {
		return existing, nil
	}

	if opts.Recipe == "hex" {
		value, err := generateHex(length / 2)
		if err != nil {
			return "", err
		}
		if err := s.Put(ctx, key, value); err != nil {
			return "", err
		}
		return value, nil
	}

	recipe := opts.Recipe
	if recipe == "" {
		recipe = fmt.Sprintf("letters,digits,%d", length)
	}

	out, err := s.Runner.Run(ctx, "op", "item", "create",
		"--category=password",
		"--title="+key,
		"--vault="+s.Vault,
		fmt.Sprintf("--generate-password=%s", recipe),
		"--format=json",
	)
	if err != nil {
		return "", fmt.Errorf("generating secret %s via op: %w", key, err)
	}

	value, err := extractPassword(out)
	if err != nil {
		return "", err
	}

	if err := s.Env.Put(ctx, key, value); err != nil {
		return "", err
	}

	return value, nil
}

func (s *OnePasswordStore) Delete(ctx context.Context, key string) error {
	s.Runner.Run(ctx, "op", "item", "delete", key, "--vault", s.Vault)
	return s.Env.Delete(ctx, key)
}

func extractPassword(opJSON []byte) (string, error) {
	var item struct {
		Fields []struct {
			ID    string `json:"id"`
			Value string `json:"value"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(opJSON, &item); err != nil {
		return "", fmt.Errorf("parsing op response: %w", err)
	}
	for _, f := range item.Fields {
		if strings.EqualFold(f.ID, "password") {
			return f.Value, nil
		}
	}
	return "", fmt.Errorf("no password field in op response")
}
