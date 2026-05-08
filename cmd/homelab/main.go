package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-isatty"

	"github.com/caboose-ai/caboose-ai.io/internal/cli"
	"github.com/caboose-ai/caboose-ai.io/internal/config"
	"github.com/caboose-ai/caboose-ai.io/internal/docker"
	"github.com/caboose-ai/caboose-ai.io/internal/install"
	"github.com/caboose-ai/caboose-ai.io/internal/migrate"
	"github.com/caboose-ai/caboose-ai.io/internal/runner"
	"github.com/caboose-ai/caboose-ai.io/internal/secrets"
	appTUI "github.com/caboose-ai/caboose-ai.io/internal/tui"
)

func main() {
	subcmd, args := extractSubcommand(os.Args[1:])

	if subcmd == "" {
		fmt.Fprintln(os.Stderr, "Usage: homelab <command> [flags]")
		fmt.Fprintln(os.Stderr, "\nCommands:")
		fmt.Fprintln(os.Stderr, "  install    Bootstrap the homelab SSO stack")
		fmt.Fprintln(os.Stderr, "  reset      Tear down everything and delete all secrets")
		fmt.Fprintln(os.Stderr, "  migrate    Migrate host services to Docker containers")
		fmt.Fprintln(os.Stderr, "  oauth-setup Print external OAuth setup and optionally create Turnstile")
		fmt.Fprintln(os.Stderr, "  service    Inspect or configure one registered service")
		os.Exit(1)
	}

	fs := flag.NewFlagSet(subcmd, flag.ExitOnError)
	var opts cliOpts
	fs.BoolVar(&opts.dryRun, "dry-run", false, "Print what would happen without making changes")
	fs.BoolVar(&opts.force, "force", false, "Force re-creation of existing resources")
	fs.BoolVar(&opts.verbose, "verbose", false, "Show detailed output")
	fs.BoolVar(&opts.nonInteractive, "non-interactive", false, "Run without TUI prompts")
	fs.StringVar(&opts.configPath, "config", "", "Path to YAML config file")
	fs.StringVar(&opts.domain, "domain", "", "Homelab domain (e.g. caboose-ai.io)")
	fs.StringVar(&opts.composeDir, "compose-dir", "", "Path to docker-compose.yml directory")
	fs.StringVar(&opts.orchestrator, "orchestrator", "", "Backend orchestrator: compose (default) or kubernetes")
	fs.StringVar(&opts.serveMode, "serve-mode", "", "Exposure mode: public (loopback behind Caddy) or local (LAN-bound ports)")
	fs.StringVar(&opts.kubeNamespace, "kube-namespace", "", "Kubernetes namespace for homelab resources (default: homelab)")
	fs.StringVar(&opts.kubeconfig, "kubeconfig", "", "Path to kubeconfig file for kubectl")
	fs.StringVar(&opts.kubeContext, "kube-context", "", "Kubernetes context name for kubectl")
	fs.StringVar(&opts.opVault, "op-vault", "", "1Password dynamic vault name")
	fs.StringVar(&opts.opStaticVault, "op-static-vault", "", "1Password static vault name")
	fs.StringVar(&opts.n8nUser, "n8n-user", "", "N8N admin username")
	fs.StringVar(&opts.email, "email", "", "Admin email for Authentik bootstrap")
	fs.BoolVar(&opts.keepEnv, "keep-env", false, "Preserve .env file during reset")
	fs.BoolVar(&opts.secretsEnvOnly, "secrets-env-only", false, "Use only compose .env for secrets and skip 1Password login checks")
	fs.BoolVar(&opts.createTurnstile, "create-turnstile", false, "Create a Cloudflare Turnstile widget via API")
	fs.StringVar(&opts.cloudflareAccountID, "cloudflare-account-id", "", "Cloudflare account ID (or CLOUDFLARE_ACCOUNT_ID)")
	fs.StringVar(&opts.cloudflareAPIToken, "cloudflare-api-token", "", "Cloudflare API token (or CLOUDFLARE_API_TOKEN)")
	fs.StringVar(&opts.turnstileWidgetName, "turnstile-widget-name", "", "Cloudflare Turnstile widget name")
	fs.Parse(args)

	switch subcmd {
	case "install":
		os.Exit(runInstall(opts))
	case "reset":
		os.Exit(runReset(opts))
	case "migrate":
		os.Exit(runMigrate(opts))
	case "oauth-setup":
		os.Exit(runOAuthSetup(opts))
	case "service":
		os.Exit(runService(opts, fs.Args()))
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", subcmd)
		os.Exit(1)
	}
}

func extractSubcommand(args []string) (string, []string) {
	known := map[string]bool{"install": true, "reset": true, "migrate": true, "oauth-setup": true, "service": true}
	if len(args) == 0 {
		return "", nil
	}
	if known[args[0]] {
		return args[0], args[1:]
	}
	return "", args
}

func runService(opts cliOpts, args []string) int {
	cfg := config.DefaultConfig()
	if opts.configPath != "" {
		loaded, err := config.LoadFromFile(opts.configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			return 1
		}
		cfg = loaded
	}
	if opts.domain != "" {
		cfg.Domain = opts.domain
	}
	if opts.composeDir != "" {
		cfg.ComposeDir = opts.composeDir
	}
	if opts.orchestrator != "" {
		cfg.Orchestrator = opts.orchestrator
	}
	if opts.serveMode != "" {
		cfg.ServeMode = opts.serveMode
	}
	if opts.opVault != "" {
		cfg.OPVault = opts.opVault
	}
	if opts.opStaticVault != "" {
		cfg.OPStaticVault = opts.opStaticVault
	}
	cfg.DryRun = opts.dryRun
	cfg.Force = opts.force
	cfg.Verbose = opts.verbose
	cfg.SecretsEnvOnly = opts.secretsEnvOnly

	cmdRunner := runner.NewLocalRunner()
	secretStore := secretStoreForConfig(cfg, cmdRunner)
	return cli.RunServiceCommand(context.Background(), cfg, cmdRunner, secretStore, args, cli.ServiceCommandOptions{DryRun: opts.dryRun, Force: opts.force})
}

type cliOpts struct {
	configPath     string
	dryRun         bool
	force          bool
	verbose        bool
	nonInteractive bool
	domain         string
	composeDir     string
	opVault        string
	opStaticVault  string
	n8nUser        string
	email          string
	orchestrator   string
	serveMode      string
	kubeNamespace  string
	kubeconfig     string
	kubeContext    string
	keepEnv        bool
	secretsEnvOnly bool

	createTurnstile     bool
	cloudflareAccountID string
	cloudflareAPIToken  string
	turnstileWidgetName string
}

func runInstall(opts cliOpts) int {
	var cfg *config.Config

	if opts.configPath != "" {
		var err error
		cfg, err = config.LoadFromFile(opts.configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			return 1
		}
	} else {
		cfg = config.DefaultConfig()
	}

	if opts.domain != "" {
		cfg.Domain = opts.domain
	}
	if opts.composeDir != "" {
		cfg.ComposeDir = opts.composeDir
	}
	if opts.orchestrator != "" {
		cfg.Orchestrator = opts.orchestrator
	}
	if opts.serveMode != "" {
		cfg.ServeMode = opts.serveMode
	}
	if opts.kubeNamespace != "" {
		cfg.Kubernetes.Namespace = opts.kubeNamespace
	}
	if opts.kubeconfig != "" {
		cfg.Kubernetes.Kubeconfig = opts.kubeconfig
	}
	if opts.kubeContext != "" {
		cfg.Kubernetes.Context = opts.kubeContext
	}
	if opts.opVault != "" {
		cfg.OPVault = opts.opVault
	}
	if opts.opStaticVault != "" {
		cfg.OPStaticVault = opts.opStaticVault
	}
	if opts.n8nUser != "" {
		cfg.N8NUser = opts.n8nUser
	}
	if opts.email != "" {
		cfg.Email = opts.email
	}

	cfg.DryRun = opts.dryRun
	cfg.Force = opts.force
	cfg.Verbose = opts.verbose
	cfg.NonInteractive = opts.nonInteractive
	cfg.SecretsEnvOnly = opts.secretsEnvOnly

	if opts.nonInteractive {
		if err := cfg.Validate(); err != nil {
			fmt.Fprintf(os.Stderr, "Config validation error: %v\n", err)
			return 1
		}
	}

	cmdRunner := runner.NewLocalRunner()
	httpClient := runner.NewHTTPClient()
	secretStore := secretStoreForConfig(cfg, cmdRunner)

	inst := install.New(cfg, secretStore, cmdRunner, httpClient)

	if opts.nonInteractive || !isatty.IsTerminal(os.Stdout.Fd()) {
		if !opts.nonInteractive {
			fmt.Fprintln(os.Stderr, "No TTY detected, falling back to non-interactive mode.")
		}
		return cli.RunInstall(context.Background(), inst)
	}

	app := appTUI.NewApp(inst)
	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
		return 1
	}
	return 0
}

func runReset(opts cliOpts) int {
	var cfg *config.Config

	if opts.configPath != "" {
		var err error
		cfg, err = config.LoadFromFile(opts.configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			return 1
		}
	} else {
		cfg = config.DefaultConfig()
	}

	if opts.composeDir != "" {
		cfg.ComposeDir = opts.composeDir
	}
	if opts.orchestrator != "" {
		cfg.Orchestrator = opts.orchestrator
	}
	if opts.serveMode != "" {
		cfg.ServeMode = opts.serveMode
	}
	if opts.kubeNamespace != "" {
		cfg.Kubernetes.Namespace = opts.kubeNamespace
	}
	if opts.kubeconfig != "" {
		cfg.Kubernetes.Kubeconfig = opts.kubeconfig
	}
	if opts.kubeContext != "" {
		cfg.Kubernetes.Context = opts.kubeContext
	}
	if opts.opVault != "" {
		cfg.OPVault = opts.opVault
	}
	if opts.opStaticVault != "" {
		cfg.OPStaticVault = opts.opStaticVault
	}
	cfg.DryRun = opts.dryRun
	cfg.SecretsEnvOnly = opts.secretsEnvOnly

	cmdRunner := runner.NewLocalRunner()
	httpClient := runner.NewHTTPClient()
	secretStore := secretStoreForConfig(cfg, cmdRunner)

	inst := install.New(cfg, secretStore, cmdRunner, httpClient)
	inst.State.KeepEnv = opts.keepEnv
	return cli.RunReset(context.Background(), inst)
}

func secretStoreForConfig(cfg *config.Config, r runner.CommandRunner) secrets.SecretStore {
	if cfg.SecretsEnvOnly {
		return secrets.NewEnvFileStore(cfg.EnvPath())
	}
	return secrets.NewSplitOnePasswordStore(cfg.OPStaticVault, cfg.OPVault, r, cfg.EnvPath())
}

func runMigrate(opts cliOpts) int {
	var cfg *config.Config

	if opts.configPath != "" {
		var err error
		cfg, err = config.LoadFromFile(opts.configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			return 1
		}
	} else {
		cfg = config.DefaultConfig()
	}

	if opts.composeDir != "" {
		cfg.ComposeDir = opts.composeDir
	}

	cmdRunner := runner.NewLocalRunner()
	compose := docker.NewComposeClient(cmdRunner, cfg.ComposeDir)
	dockerExec := docker.NewExecClient(cmdRunner)

	m := migrate.NewMattermost(cmdRunner, compose, dockerExec)
	m.ProgressFn = func(s migrate.Step) {
		switch s.Status {
		case "running":
			fmt.Fprintf(os.Stderr, "● %s...\n", s.Name)
		case "done":
			fmt.Fprintf(os.Stderr, "✓ %s\n", s.Name)
		case "skipped":
			fmt.Fprintf(os.Stderr, "– %s (skipped)\n", s.Name)
		case "failed":
			fmt.Fprintf(os.Stderr, "✗ %s: %v\n", s.Name, s.Err)
		}
	}

	if err := m.Run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "migration failed: %v\n", err)
		return 1
	}

	fmt.Fprintln(os.Stderr, "✓ migration complete")
	return 0
}

func runOAuthSetup(opts cliOpts) int {
	var cfg *config.Config

	if opts.configPath != "" {
		var err error
		cfg, err = config.LoadFromFile(opts.configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			return 1
		}
	} else {
		cfg = config.DefaultConfig()
	}

	if opts.domain != "" {
		cfg.Domain = opts.domain
	}
	if cfg.Domain == "" {
		fmt.Fprintln(os.Stderr, "Config validation error: domain is required")
		return 1
	}

	if opts.cloudflareAccountID == "" {
		opts.cloudflareAccountID = os.Getenv("CLOUDFLARE_ACCOUNT_ID")
	}
	if opts.cloudflareAPIToken == "" {
		opts.cloudflareAPIToken = os.Getenv("CLOUDFLARE_API_TOKEN")
	}

	_, err := cli.RunOAuthSetup(context.Background(), os.Stdout, cfg, cli.OAuthSetupOptions{
		CreateTurnstile:     opts.createTurnstile,
		CloudflareAccountID: opts.cloudflareAccountID,
		CloudflareAPIToken:  opts.cloudflareAPIToken,
		TurnstileWidgetName: opts.turnstileWidgetName,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "oauth setup failed: %v\n", err)
		return 1
	}
	return 0
}
