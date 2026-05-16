package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/caboose-ai/caboose-ai.io/internal/runner"
)

type OnePasswordStore struct {
	Vault        string
	StaticVault  string
	DynamicVault string
	Runner       runner.CommandRunner
	Env          *EnvFileStore
}

func NewOnePasswordStore(vault string, r runner.CommandRunner, envPath string) *OnePasswordStore {
	return NewSplitOnePasswordStore(vault, vault, r, envPath)
}

func NewSplitOnePasswordStore(staticVault, dynamicVault string, r runner.CommandRunner, envPath string) *OnePasswordStore {
	if staticVault == "" {
		staticVault = dynamicVault
	}
	if dynamicVault == "" {
		dynamicVault = staticVault
	}
	return &OnePasswordStore{
		Vault:        dynamicVault,
		StaticVault:  staticVault,
		DynamicVault: dynamicVault,
		Runner:       r,
		Env:          NewEnvFileStore(envPath),
	}
}

func (s *OnePasswordStore) EnsureVault(ctx context.Context) error {
	seen := map[string]bool{}
	for _, vault := range []string{s.StaticVault, s.DynamicVault} {
		if vault == "" || seen[vault] {
			continue
		}
		seen[vault] = true
		if err := s.ensureVault(ctx, vault); err != nil {
			return err
		}
	}
	return nil
}

func (s *OnePasswordStore) ensureVault(ctx context.Context, vault string) error {
	_, err := s.Runner.Run(ctx, "op", "vault", "get", vault, "--format=json")
	if err == nil {
		return nil
	}
	_, err = s.Runner.Run(ctx, "op", "vault", "create", vault)
	if err != nil {
		return fmt.Errorf("creating 1Password vault %q: %w", vault, err)
	}
	return nil
}

func (s *OnePasswordStore) Get(ctx context.Context, key string) (string, error) {
	vault := s.vaultForKey(key)
	out, err := s.Runner.Run(ctx, "op", "item", "get", key,
		"--vault", vault,
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
		if err := s.Env.Put(ctx, key, field.Value); err != nil {
			return "", fmt.Errorf("sync 1Password secret %s to env: %w", key, err)
		}
	}
	return field.Value, nil
}

func (s *OnePasswordStore) Put(ctx context.Context, key, value string) error {
	if err := s.Env.Put(ctx, key, value); err != nil {
		return err
	}

	vault := s.vaultForKey(key)
	existing, _ := s.Runner.Run(ctx, "op", "item", "get", key,
		"--vault", vault, "--format=json")

	if existing != nil {
		_, err := s.Runner.Run(ctx, "op", "item", "edit", key,
			"password="+value,
			"--vault", vault,
		)
		return err
	}

	_, err := s.Runner.Run(ctx, "op", "item", "create",
		"--category=password",
		"--title="+key,
		"--vault="+vault,
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
		"--vault="+s.DynamicVault,
		fmt.Sprintf("--generate-password=%s", recipe),
		"--format=json",
	)
	if err != nil {
		existing, getErr := s.Get(ctx, key)
		if getErr == nil && existing != "" {
			return existing, nil
		}
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
	vault := s.vaultForKey(key)
	existing, err := s.Runner.Run(ctx, "op", "item", "get", key,
		"--vault", vault,
		"--format=json",
	)
	if err != nil {
		return s.Env.Delete(ctx, key)
	}

	var item struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(existing, &item); err != nil {
		return fmt.Errorf("parsing 1Password item %q before delete: %w", key, err)
	}
	if item.ID == "" {
		return fmt.Errorf("1Password item %q response omitted id", key)
	}

	if _, err := s.Runner.Run(ctx, "op", "item", "delete", item.ID, "--vault", vault); err != nil {
		return fmt.Errorf("deleting 1Password item %q: %w", key, err)
	}
	return s.Env.Delete(ctx, key)
}

func (s *OnePasswordStore) vaultForKey(key string) string {
	for _, staticKey := range StaticSecretKeys() {
		if key == staticKey {
			return s.StaticVault
		}
	}
	return s.DynamicVault
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
