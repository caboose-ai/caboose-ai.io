package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
	"github.com/caboose-ai/caboose-ai.io/internal/mcpaccess"
	"github.com/caboose-ai/caboose-ai.io/internal/runner"
	"github.com/caboose-ai/caboose-ai.io/internal/secrets"
	"github.com/caboose-ai/caboose-ai.io/services/authentik"
)

type MCPAccessAdminOptions struct {
	Config   *config.Config
	Secrets  secrets.SecretStore
	HTTP     runner.HTTPClient
	Stdout   io.Writer
	Stderr   io.Writer
	AdminURL string
}

func RunMCPAccessAdmin(ctx context.Context, opts MCPAccessAdminOptions, args []string) int {
	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}
	if len(args) == 0 {
		printMCPAccessAdminUsage(stderr)
		return 1
	}

	switch args[0] {
	case "setup":
		return runMCPAccessSetup(ctx, stdout, stderr, opts, args[1:])
	case "approve":
		return runMCPAccessApprove(ctx, stdout, stderr, opts, args[1:])
	case "-h", "--help", "help":
		printMCPAccessAdminUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown mcp access command %q\n", args[0])
		printMCPAccessAdminUsage(stderr)
		return 1
	}
}

func runMCPAccessSetup(ctx context.Context, stdout, stderr io.Writer, opts MCPAccessAdminOptions, args []string) int {
	fs := flag.NewFlagSet("homelab mcp access setup", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 1
	}
	ak, authURL, err := authentikAdminClient(ctx, opts)
	if err != nil {
		fmt.Fprintf(stderr, "mcp access setup failed: %v\n", err)
		return 1
	}

	mapping, err := ak.GetOAuthScopeMappingByScopeName(ctx, mcpaccess.ScopeHomelab)
	if err != nil {
		fmt.Fprintf(stderr, "looking up homelab scope mapping: %v\n", err)
		return 1
	}
	if mapping == nil {
		mapping, err = ak.CreateOAuthScopeMapping(ctx, authentik.CreateOAuthScopeMappingParams{
			Name:        "Homelab MCP",
			ScopeName:   mcpaccess.ScopeHomelab,
			Description: "Homelab MCP access scope",
			Expression:  "return {}",
		})
		if err != nil {
			fmt.Fprintf(stderr, "creating homelab scope mapping: %v\n", err)
			return 1
		}
	}

	authFlow, err := ensureProviderFlow(ctx, ak, authentik.CreateFlowParams{
		Name:        "Provider authorization implicit consent",
		Slug:        "default-provider-authorization-implicit-consent",
		Title:       "Redirecting to application",
		Designation: "authorization",
	})
	if err != nil {
		fmt.Fprintf(stderr, "ensuring authorization flow: %v\n", err)
		return 1
	}
	invalidationFlow, err := ensureProviderFlow(ctx, ak, authentik.CreateFlowParams{
		Name:        "Provider invalidation flow",
		Slug:        "default-provider-invalidation-flow",
		Title:       "You've logged out of the application.",
		Designation: "invalidation",
	})
	if err != nil {
		fmt.Fprintf(stderr, "ensuring invalidation flow: %v\n", err)
		return 1
	}
	signingKey, err := providerSigningKey(ctx, ak)
	if err != nil {
		fmt.Fprintf(stderr, "finding provider signing key: %v\n", err)
		return 1
	}

	provider, err := ak.GetProvider(ctx, "homelab-mcp")
	if err != nil {
		fmt.Fprintf(stderr, "looking up Homelab MCP provider: %v\n", err)
		return 1
	}
	if provider == nil {
		provider, err = ak.CreateProvider(ctx, authentik.CreateProviderParams{
			Name:                "Homelab MCP",
			AuthorizationFlow:   authFlow.PK,
			InvalidationFlow:    invalidationFlow.PK,
			ClientType:          "confidential",
			RedirectURIs:        []authentik.RedirectURI{},
			SigningKey:          signingKey,
			PropertyMappings:    []string{mapping.PK},
			AccessTokenValidity: "hours=1",
		})
		if err != nil {
			fmt.Fprintf(stderr, "creating Homelab MCP provider: %v\n", err)
			return 1
		}
	} else if err := ak.UpdateProviderMCPSettings(ctx, provider.PK, []string{mapping.PK}, signingKey); err != nil {
		fmt.Fprintf(stderr, "updating Homelab MCP provider: %v\n", err)
		return 1
	}

	if app, err := ak.GetApplication(ctx, "homelab-mcp"); err != nil {
		fmt.Fprintf(stderr, "looking up Homelab MCP application: %v\n", err)
		return 1
	} else if app == nil {
		if _, err := ak.CreateApplication(ctx, "Homelab MCP", "homelab-mcp", provider.PK); err != nil {
			fmt.Fprintf(stderr, "creating Homelab MCP application: %v\n", err)
			return 1
		}
	} else if err := ak.EnsureApplicationOpen(ctx, "homelab-mcp"); err != nil {
		fmt.Fprintf(stderr, "opening Homelab MCP application policy: %v\n", err)
		return 1
	}

	if provider.ClientID == "" || provider.ClientSecret == "" {
		fmt.Fprintln(stderr, "Homelab MCP provider response omitted client credentials")
		return 1
	}
	if err := opts.Secrets.Put(ctx, "MCP_OAUTH_CLIENT_ID", provider.ClientID); err != nil {
		fmt.Fprintf(stderr, "storing MCP client id: %v\n", err)
		return 1
	}
	if err := opts.Secrets.Put(ctx, "MCP_OAUTH_CLIENT_SECRET", provider.ClientSecret); err != nil {
		fmt.Fprintf(stderr, "storing MCP client secret: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Homelab MCP OAuth provider ready at %s\n", authURL)
	return 0
}

func runMCPAccessApprove(ctx context.Context, stdout, stderr io.Writer, opts MCPAccessAdminOptions, args []string) int {
	fs := flag.NewFlagSet("homelab mcp access approve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	out := fs.String("out", "-", "Output release path, or - for stdout")
	expires := fs.Duration("expires", 8760*time.Hour, "Service account token lifetime")
	requestPath := ""
	parseArgs := args
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		requestPath = args[0]
		parseArgs = args[1:]
	}
	if err := fs.Parse(parseArgs); err != nil {
		return 1
	}
	if requestPath == "" && fs.NArg() == 1 {
		requestPath = fs.Arg(0)
	}
	if requestPath == "" || fs.NArg() > 1 {
		fmt.Fprintln(stderr, "Usage: homelab mcp access approve [flags] <request.json>")
		return 1
	}
	ak, authURL, err := authentikAdminClient(ctx, opts)
	if err != nil {
		fmt.Fprintf(stderr, "mcp access approve failed: %v\n", err)
		return 1
	}
	var req mcpaccess.AccessRequest
	if err := readJSONPath(requestPath, &req); err != nil {
		fmt.Fprintf(stderr, "reading access request: %v\n", err)
		return 1
	}
	clientID, err := opts.Secrets.Get(ctx, "MCP_OAUTH_CLIENT_ID")
	if err != nil || clientID == "" {
		fmt.Fprintln(stderr, "MCP_OAUTH_CLIENT_ID is required; run homelab mcp access setup first")
		return 1
	}

	accountName := "mcp-" + slugify(req.Name)
	expiring := *expires > 0
	var expiresAt time.Time
	if expiring {
		expiresAt = time.Now().UTC().Add(*expires)
	}
	account, err := ak.CreateServiceAccount(ctx, authentik.CreateServiceAccountParams{
		Name:        accountName,
		CreateGroup: false,
		Expiring:    expiring,
		Expires:     expiresAt,
	})
	if err != nil {
		fmt.Fprintf(stderr, "creating service account: %v\n", err)
		return 1
	}
	release, err := mcpaccess.SealRelease(&req, mcpaccess.Credential{
		Endpoint:     req.Endpoint,
		AuthentikURL: authURL,
		ClientID:     clientID,
		Username:     account.Username,
		AppPassword:  account.Token,
		Scope:        req.Scope,
		ExpiresAt:    expiresAt,
	}, mcpaccess.ReleaseOptions{})
	if err != nil {
		fmt.Fprintf(stderr, "sealing release: %v\n", err)
		return 1
	}
	if err := writeJSONOutput(stdout, *out, release); err != nil {
		fmt.Fprintf(stderr, "writing release: %v\n", err)
		return 1
	}
	if *out != "-" {
		fmt.Fprintf(stdout, "release=%s\n", *out)
	}
	return 0
}

func authentikAdminClient(ctx context.Context, opts MCPAccessAdminOptions) (*authentik.Client, string, error) {
	if opts.Config == nil {
		return nil, "", fmt.Errorf("config is required")
	}
	if opts.Secrets == nil {
		return nil, "", fmt.Errorf("secret store is required")
	}
	token, err := opts.Secrets.Get(ctx, "AUTHENTIK_BOOTSTRAP_TOKEN")
	if err != nil || token == "" {
		return nil, "", fmt.Errorf("AUTHENTIK_BOOTSTRAP_TOKEN is required")
	}
	authURL := opts.AdminURL
	if authURL == "" {
		authURL = opts.Config.URLs().Authentik
	}
	if authURL == "" {
		return nil, "", fmt.Errorf("authentik URL is required")
	}
	httpClient := opts.HTTP
	if httpClient == nil {
		httpClient = runner.NewHTTPClient()
	}
	return authentik.NewClient(authURL, token, httpClient), authURL, nil
}

func ensureProviderFlow(ctx context.Context, ak *authentik.Client, params authentik.CreateFlowParams) (*authentik.Flow, error) {
	flow, err := ak.GetFlow(ctx, params.Slug)
	if err == nil {
		return flow, nil
	}
	if !strings.Contains(err.Error(), "not found") {
		return nil, err
	}
	flow, err = ak.CreateFlow(ctx, params)
	if err == nil {
		return flow, nil
	}
	if strings.Contains(err.Error(), "already exists") {
		return ak.GetFlow(ctx, params.Slug)
	}
	return nil, err
}

func providerSigningKey(ctx context.Context, ak *authentik.Client) (string, error) {
	keys, err := ak.ListCertificateKeyPairs(ctx)
	if err != nil {
		return "", err
	}
	for _, key := range keys {
		if key.Name == "authentik Internal JWT Certificate" {
			return key.PK, nil
		}
	}
	if len(keys) > 0 {
		return keys[0].PK, nil
	}
	return "", nil
}

func printMCPAccessAdminUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: homelab mcp access <setup|approve> [flags]")
}

func readJSONPath(path string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, value)
}

func writeJSONPath(path string, value any, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, mode)
}

func slugify(value string) string {
	var out []rune
	lastDash := false
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			out = append(out, r)
			lastDash = false
			continue
		}
		if !lastDash && len(out) > 0 {
			out = append(out, '-')
			lastDash = true
		}
	}
	for len(out) > 0 && out[len(out)-1] == '-' {
		out = out[:len(out)-1]
	}
	if len(out) == 0 {
		return "client"
	}
	return string(out)
}
