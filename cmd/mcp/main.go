package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/caboose-ai/caboose-ai.io/internal/cli"
	"github.com/caboose-ai/caboose-ai.io/internal/config"
	mcpserver "github.com/caboose-ai/caboose-ai.io/internal/mcp"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "access" {
		os.Exit(cli.RunMCPAccessClient(context.Background(), os.Stdout, os.Stderr, os.Args[2:], cli.MCPAccessClientOptions{}))
	}

	var (
		configPath string
		httpAddr   string
	)
	fs := flag.NewFlagSet("homelab-mcp", flag.ExitOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), mcpUsage())
		fs.PrintDefaults()
	}
	fs.StringVar(&configPath, "config", "", "Path to homelab.yml config file (required)")
	fs.StringVar(&httpAddr, "http", "", "Listen address for HTTP mode (e.g. :8090). If unset, uses stdio transport.")
	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}

	if configPath == "" {
		fmt.Fprint(os.Stderr, mcpUsage())
		os.Exit(1)
	}

	cfg, err := config.LoadFromFile(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Config validation error: %v\n", err)
		os.Exit(1)
	}

	srv := mcpserver.New(cfg)

	if httpAddr != "" {
		if err := srv.RunHTTP(httpAddr); err != nil {
			fmt.Fprintf(os.Stderr, "HTTP server error: %v\n", err)
			os.Exit(1)
		}
	} else {
		if err := srv.Run(context.Background()); err != nil {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
			os.Exit(1)
		}
	}
}

func mcpUsage() string {
	return `Usage:
  homelab-mcp --config /path/to/homelab.yml [--http :8090]
  homelab-mcp access <request|import|token|status> [flags]

MCP access startup flow:
  homelab mcp access setup
  homelab-mcp access request --name "$(hostname)" --out mcp-request.json
  homelab mcp access approve mcp-request.json --out mcp-release.json
  homelab-mcp access import mcp-release.json
  homelab-mcp access status

Server flags:
`
}
