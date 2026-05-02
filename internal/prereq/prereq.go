package prereq

import (
	"context"
	"fmt"

	"github.com/caboose-ai/caboose-ai.io/internal/runner"
)

type Result struct {
	Name    string
	Found   bool
	Version string
	Err     error
}

type Checker struct {
	Runner runner.CommandRunner
}

func NewChecker(r runner.CommandRunner) *Checker {
	return &Checker{Runner: r}
}

var required = []struct {
	name       string
	cmd        string
	versionArg string
}{
	{"docker", "docker", "--version"},
	{"docker compose", "docker", "compose version"},
	{"op (1Password CLI)", "op", "--version"},
}

func (c *Checker) CheckAll(ctx context.Context) []Result {
	results := make([]Result, len(required))
	for i, req := range required {
		results[i] = c.check(ctx, req.name, req.cmd, req.versionArg)
	}
	return results
}

func (c *Checker) check(ctx context.Context, name, cmd, versionArg string) Result {
	args := []string{versionArg}
	if cmd == "docker" && versionArg == "compose version" {
		args = []string{"compose", "version"}
	}

	out, err := c.Runner.Run(ctx, cmd, args...)
	if err != nil {
		return Result{Name: name, Err: fmt.Errorf("%s not found: %w", name, err)}
	}
	return Result{Name: name, Found: true, Version: string(out)}
}

func AllPassed(results []Result) bool {
	for _, r := range results {
		if !r.Found {
			return false
		}
	}
	return true
}
