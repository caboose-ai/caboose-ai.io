package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
	mcpserver "github.com/caboose-ai/caboose-ai.io/internal/mcp"
)

func main() {
	var (
		configPath string
		httpAddr   string
	)
	flag.StringVar(&configPath, "config", "", "Path to homelab.yml config file (required)")
	flag.StringVar(&httpAddr, "http", "", "Listen address for HTTP mode (e.g. :8090). If unset, uses stdio transport.")
	flag.Parse()

	if configPath == "" {
		fmt.Fprintln(os.Stderr, "Usage: homelab-mcp --config /path/to/homelab.yml [--http :8090]")
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
