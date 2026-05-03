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
	subcmd, args := extractSubcommand(os.Args[1:])

	if subcmd == "" {
		fmt.Fprintln(os.Stderr, "Usage: homelab <command> [flags]")
		fmt.Fprintln(os.Stderr, "\nCommands:")
		fmt.Fprintln(os.Stderr, "  install    Bootstrap the homelab SSO stack")
		fmt.Fprintln(os.Stderr, "  reset      Tear down everything and delete all secrets")
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
	fs.StringVar(&opts.opVault, "op-vault", "", "1Password vault name")
	fs.StringVar(&opts.n8nUser, "n8n-user", "", "N8N admin username")
	fs.StringVar(&opts.email, "email", "", "Admin email for Authentik bootstrap")
	fs.Parse(args)

	switch subcmd {
	case "install":
		os.Exit(runInstall(opts))
	case "reset":
		os.Exit(runReset(opts))
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", subcmd)
		os.Exit(1)
	}
}

func extractSubcommand(args []string) (string, []string) {
	known := map[string]bool{"install": true, "reset": true}
	var rest []string
	var subcmd string
	for _, arg := range args {
		if subcmd == "" && known[arg] {
			subcmd = arg
		} else {
			rest = append(rest, arg)
		}
	}
	return subcmd, rest
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
	if opts.opVault != "" {
		cfg.OPVault = opts.opVault
	}
	cfg.DryRun = opts.dryRun

	cmdRunner := runner.NewLocalRunner()
	httpClient := runner.NewHTTPClient()
	secretStore := secrets.NewOnePasswordStore(cfg.OPVault, cmdRunner, cfg.EnvPath())

	inst := install.New(cfg, secretStore, cmdRunner, httpClient)
	return cli.RunReset(context.Background(), inst)
}
