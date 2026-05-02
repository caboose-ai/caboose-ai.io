package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/caboose-ai/caboose-ai.io/internal/cli"
	"github.com/caboose-ai/caboose-ai.io/internal/config"
	"github.com/caboose-ai/caboose-ai.io/internal/install"
	"github.com/caboose-ai/caboose-ai.io/internal/runner"
	"github.com/caboose-ai/caboose-ai.io/internal/secrets"
	appTUI "github.com/caboose-ai/caboose-ai.io/internal/tui"
)

func main() {
	var (
		dryRun         bool
		force          bool
		verbose        bool
		nonInteractive bool
		configPath     string
		domain         string
		composeDir     string
		opVault        string
		n8nUser        string
		email          string
	)

	flag.BoolVar(&dryRun, "dry-run", false, "Print what would happen without making changes")
	flag.BoolVar(&force, "force", false, "Force re-creation of existing resources")
	flag.BoolVar(&verbose, "verbose", false, "Show detailed output")
	flag.BoolVar(&nonInteractive, "non-interactive", false, "Run without TUI prompts")
	flag.StringVar(&configPath, "config", "", "Path to YAML config file")
	flag.StringVar(&domain, "domain", "", "Homelab domain (e.g. caboose-ai.io)")
	flag.StringVar(&composeDir, "compose-dir", "", "Path to docker-compose.yml directory")
	flag.StringVar(&opVault, "op-vault", "", "1Password vault name")
	flag.StringVar(&n8nUser, "n8n-user", "", "N8N admin username")
	flag.StringVar(&email, "email", "", "Admin email for Authentik bootstrap")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: homelab <command> [flags]")
		fmt.Fprintln(os.Stderr, "\nCommands:")
		fmt.Fprintln(os.Stderr, "  install    Bootstrap the homelab SSO stack")
		fmt.Fprintln(os.Stderr, "\nFlags:")
		flag.PrintDefaults()
		os.Exit(1)
	}

	switch args[0] {
	case "install":
		os.Exit(runInstall(cliOpts{
			configPath:     configPath,
			dryRun:         dryRun,
			force:          force,
			verbose:        verbose,
			nonInteractive: nonInteractive,
			domain:         domain,
			composeDir:     composeDir,
			opVault:        opVault,
			n8nUser:        n8nUser,
			email:          email,
		}))
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", args[0])
		os.Exit(1)
	}
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
	n8nUser        string
	email          string
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
	if opts.opVault != "" {
		cfg.OPVault = opts.opVault
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

	if opts.nonInteractive {
		if err := cfg.Validate(); err != nil {
			fmt.Fprintf(os.Stderr, "Config validation error: %v\n", err)
			return 1
		}
	}

	cmdRunner := runner.NewLocalRunner()
	httpClient := runner.NewHTTPClient()
	secretStore := secrets.NewOnePasswordStore(cfg.OPVault, cmdRunner, cfg.EnvPath())

	inst := install.New(cfg, secretStore, cmdRunner, httpClient)

	if opts.nonInteractive {
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
