package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/caboose-ai/caboose-ai.io/internal/mcpaccess"
)

type MCPAccessClientOptions struct {
	HTTPClient *http.Client
}

func RunMCPAccessClient(ctx context.Context, stdout, stderr io.Writer, args []string, opts MCPAccessClientOptions) int {
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	if len(args) == 0 {
		printMCPAccessClientUsage(stderr)
		return 1
	}

	switch args[0] {
	case "request":
		return runMCPAccessRequest(stdout, stderr, args[1:])
	case "import":
		return runMCPAccessImport(stdout, stderr, args[1:])
	case "token":
		return runMCPAccessToken(ctx, stdout, stderr, args[1:], opts)
	case "status":
		return runMCPAccessStatus(stdout, stderr, args[1:])
	case "-h", "--help", "help":
		printMCPAccessClientUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown access command %q\n", args[0])
		printMCPAccessClientUsage(stderr)
		return 1
	}
}

func runMCPAccessRequest(stdout, stderr io.Writer, args []string) int {
	fs := flag.NewFlagSet("homelab-mcp access request", flag.ContinueOnError)
	fs.SetOutput(stderr)
	name := fs.String("name", "", "Client name to request access for")
	endpoint := fs.String("endpoint", mcpaccess.DefaultEndpoint, "MCP HTTPS endpoint")
	pendingDir := fs.String("pending-dir", "", "Directory for pending request private keys")
	out := fs.String("out", "-", "Output request path, or - for stdout")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *name == "" {
		fmt.Fprintln(stderr, "--name is required")
		return 1
	}
	req, err := mcpaccess.CreateRequest(mcpaccess.RequestOptions{
		Name:       *name,
		Endpoint:   *endpoint,
		PendingDir: *pendingDir,
	})
	if err != nil {
		fmt.Fprintf(stderr, "access request failed: %v\n", err)
		return 1
	}
	if err := writeJSONOutput(stdout, *out, req); err != nil {
		fmt.Fprintf(stderr, "writing access request: %v\n", err)
		return 1
	}
	if *out != "-" {
		fmt.Fprintf(stdout, "request=%s\n", *out)
	}
	return 0
}

func runMCPAccessImport(stdout, stderr io.Writer, args []string) int {
	fs := flag.NewFlagSet("homelab-mcp access import", flag.ContinueOnError)
	fs.SetOutput(stderr)
	pendingDir := fs.String("pending-dir", "", "Directory for pending request private keys")
	credentialFile := fs.String("credential-file", "", "Credential file path")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "Usage: homelab-mcp access import [flags] <release.json>")
		return 1
	}
	dir := *pendingDir
	if dir == "" {
		var err error
		dir, err = mcpaccess.DefaultPendingDir()
		if err != nil {
			fmt.Fprintf(stderr, "finding pending directory: %v\n", err)
			return 1
		}
	}
	var release mcpaccess.Release
	if err := readJSONPath(fs.Arg(0), &release); err != nil {
		fmt.Fprintf(stderr, "reading release: %v\n", err)
		return 1
	}
	cred, err := mcpaccess.OpenRelease(&release, dir)
	if err != nil {
		fmt.Fprintf(stderr, "opening release: %v\n", err)
		return 1
	}
	if err := mcpaccess.StoreCredential(*credentialFile, cred); err != nil {
		fmt.Fprintf(stderr, "storing credential: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "credential imported for %s\n", cred.Endpoint)
	return 0
}

func runMCPAccessToken(ctx context.Context, stdout, stderr io.Writer, args []string, opts MCPAccessClientOptions) int {
	fs := flag.NewFlagSet("homelab-mcp access token", flag.ContinueOnError)
	fs.SetOutput(stderr)
	credentialFile := fs.String("credential-file", "", "Credential file path")
	export := fs.Bool("export", false, "Print a shell export instead of the raw token")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	cred, err := mcpaccess.LoadCredential(*credentialFile)
	if err != nil {
		printMCPAccessCredentialError(stderr, *credentialFile, err)
		return 1
	}
	token, err := mcpaccess.MintToken(ctx, cred, opts.HTTPClient)
	if err != nil {
		fmt.Fprintf(stderr, "minting token: %v\n", err)
		return 1
	}
	if *export {
		fmt.Fprintf(stdout, "export HOMELAB_MCP_TOKEN=%q\n", token)
	} else {
		fmt.Fprintln(stdout, token)
	}
	return 0
}

func runMCPAccessStatus(stdout, stderr io.Writer, args []string) int {
	fs := flag.NewFlagSet("homelab-mcp access status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	credentialFile := fs.String("credential-file", "", "Credential file path")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	cred, err := mcpaccess.LoadCredential(*credentialFile)
	if err != nil {
		printMCPAccessCredentialError(stderr, *credentialFile, err)
		return 1
	}
	fmt.Fprintf(stdout, "endpoint=%s\n", cred.Endpoint)
	fmt.Fprintf(stdout, "authentik_url=%s\n", cred.AuthentikURL)
	fmt.Fprintf(stdout, "username=%s\n", cred.Username)
	fmt.Fprintf(stdout, "scope=%s\n", cred.Scope)
	if !cred.ExpiresAt.IsZero() {
		fmt.Fprintf(stdout, "expires_at=%s\n", cred.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"))
	}
	return 0
}

func printMCPAccessClientUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: homelab-mcp access <request|import|token|status> [flags]")
}

func printMCPAccessCredentialError(w io.Writer, path string, err error) {
	if !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(w, "loading credential: %v\n", err)
		return
	}
	explicitPath := path != ""
	if path == "" {
		if defaultPath, defaultErr := mcpaccess.DefaultCredentialPath(); defaultErr == nil {
			path = defaultPath
		}
	}
	credentialFlag := ""
	if explicitPath {
		credentialFlag = " --credential-file " + strconv.Quote(path)
	}
	if path == "" {
		fmt.Fprintln(w, "No Homelab MCP access credential found.")
	} else {
		fmt.Fprintf(w, "No Homelab MCP access credential found at %s.\n", path)
	}
	fmt.Fprintln(w, "Create and import one with:")
	fmt.Fprintln(w, "  homelab mcp access setup")
	fmt.Fprintln(w, "  homelab-mcp access request --name \"$(hostname)\" --out mcp-request.json")
	fmt.Fprintln(w, "  homelab mcp access approve mcp-request.json --out mcp-release.json")
	fmt.Fprintf(w, "  homelab-mcp access import%s mcp-release.json\n", credentialFlag)
	fmt.Fprintln(w, "Then retry:")
	fmt.Fprintf(w, "  homelab-mcp access status%s\n", credentialFlag)
}

func writeJSONOutput(stdout io.Writer, path string, value any) error {
	if path == "-" || path == "" {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(value)
	}
	return writeJSONPath(path, value, 0o600)
}
