package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/caboose-ai/caboose-ai.io/internal/secrets"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type secretsGetInput struct {
	Key string `json:"key" jsonschema:"the secret key to retrieve (e.g. AUTHENTIK_SECRET_KEY)"`
}

type secretsPutInput struct {
	Key   string `json:"key" jsonschema:"the secret key to store"`
	Value string `json:"value" jsonschema:"the secret value to store"`
}

type secretsGenerateInput struct {
	Key    string `json:"key" jsonschema:"the secret key to generate"`
	Length int    `json:"length,omitempty" jsonschema:"length of the generated secret (default 32)"`
	Recipe string `json:"recipe,omitempty" jsonschema:"generation recipe: hex or letters,digits,symbols (default letters,digits,<length>)"`
}

func (s *Server) registerSecretsTools() {
	sdkmcp.AddTool(s.mcpServer, &sdkmcp.Tool{
		Name:        "secrets_get",
		Description: "Retrieve a secret value by key from 1Password or .env",
	}, s.handleSecretsGet)

	sdkmcp.AddTool(s.mcpServer, &sdkmcp.Tool{
		Name:        "secrets_put",
		Description: "Store a secret value in 1Password and .env",
	}, s.handleSecretsPut)

	sdkmcp.AddTool(s.mcpServer, &sdkmcp.Tool{
		Name:        "secrets_generate",
		Description: "Generate a new random secret and store it",
	}, s.handleSecretsGenerate)

	sdkmcp.AddTool(s.mcpServer, &sdkmcp.Tool{
		Name:        "secrets_list",
		Description: "List all defined secret keys with their required/optional status",
	}, s.handleSecretsList)
}

func (s *Server) handleSecretsGet(ctx context.Context, _ *sdkmcp.CallToolRequest, input secretsGetInput) (*sdkmcp.CallToolResult, any, error) {
	if input.Key == "" {
		return nil, nil, fmt.Errorf("secrets_get: key is required")
	}
	val, err := s.secrets.Get(ctx, input.Key)
	if err != nil {
		return nil, nil, fmt.Errorf("secrets_get: %w", err)
	}
	if val == "" {
		return textResult(fmt.Sprintf("Secret %q not found", input.Key)), nil, nil
	}
	return textResult(val), nil, nil
}

func (s *Server) handleSecretsPut(ctx context.Context, _ *sdkmcp.CallToolRequest, input secretsPutInput) (*sdkmcp.CallToolResult, any, error) {
	if input.Key == "" || input.Value == "" {
		return nil, nil, fmt.Errorf("secrets_put: both key and value are required")
	}
	if err := s.secrets.Put(ctx, input.Key, input.Value); err != nil {
		return nil, nil, fmt.Errorf("secrets_put: %w", err)
	}
	return textResult(fmt.Sprintf("Secret %q stored successfully", input.Key)), nil, nil
}

func (s *Server) handleSecretsGenerate(ctx context.Context, _ *sdkmcp.CallToolRequest, input secretsGenerateInput) (*sdkmcp.CallToolResult, any, error) {
	if input.Key == "" {
		return nil, nil, fmt.Errorf("secrets_generate: key is required")
	}
	length := input.Length
	if length <= 0 {
		length = 32
	}
	recipe := input.Recipe
	if recipe == "" {
		recipe = fmt.Sprintf("letters,digits,%d", length)
	}
	val, err := s.secrets.Generate(ctx, input.Key, length, secrets.GenerateOpts{Recipe: recipe})
	if err != nil {
		return nil, nil, fmt.Errorf("secrets_generate: %w", err)
	}
	return textResult(fmt.Sprintf("Generated and stored secret %q: %s", input.Key, val)), nil, nil
}

type secretListEntry struct {
	Key    string `json:"key"`
	Prompt bool   `json:"prompt"`
}

func (s *Server) handleSecretsList(_ context.Context, _ *sdkmcp.CallToolRequest, _ any) (*sdkmcp.CallToolResult, any, error) {
	var entries []secretListEntry
	for _, def := range secrets.BootstrapSecrets() {
		entries = append(entries, secretListEntry{Key: def.Key, Prompt: def.Prompt})
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("secrets_list: %w", err)
	}
	return textResult(string(data)), nil, nil
}
