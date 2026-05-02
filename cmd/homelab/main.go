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
	)

	flag.BoolVar(&dryRun, "dry-run", false, "Print what would happen without making changes")
	flag.BoolVar(&force, "force", false, "Force re-creation of existing resources")
	flag.BoolVar(&verbose, "verbose", false, "Show detailed output")
	flag.BoolVar(&nonInteractive, "non-interactive", false, "Run without TUI prompts")
	flag.StringVar(&configPath, "config", "", "Path to YAML config file (required for --non-interactive)")
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
		os.Exit(runInstall(configPath, dryRun, force, verbose, nonInteractive))
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", args[0])
		os.Exit(1)
	}
}

func runInstall(configPath string, dryRun, force, verbose, nonInteractive bool) int {
	var cfg *config.Config

	if configPath != "" {
		var err error
		cfg, err = config.LoadFromFile(configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			return 1
		}
	} else {
		cfg = config.DefaultConfig()
	}

	cfg.DryRun = dryRun
	cfg.Force = force
	cfg.Verbose = verbose
	cfg.NonInteractive = nonInteractive

	if nonInteractive {
		if err := cfg.Validate(); err != nil {
			fmt.Fprintf(os.Stderr, "Config validation error: %v\n", err)
			return 1
		}
	}

	cmdRunner := runner.NewLocalRunner()
	httpClient := runner.NewHTTPClient()
	secretStore := secrets.NewOnePasswordStore(cfg.OPVault, cmdRunner, cfg.EnvPath())

	inst := install.New(cfg, secretStore, cmdRunner, httpClient)

	if nonInteractive {
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
