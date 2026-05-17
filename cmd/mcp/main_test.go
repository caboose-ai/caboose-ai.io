package main

import (
	"strings"
	"testing"
)

func TestUsageIncludesAccessStartupCommands(t *testing.T) {
	usage := mcpUsage()
	for _, want := range []string{
		"homelab-mcp access status",
		"homelab-mcp access request",
		"homelab-mcp access import",
		"homelab mcp access setup",
	} {
		if !strings.Contains(usage, want) {
			t.Fatalf("usage missing %q:\n%s", want, usage)
		}
	}
}
