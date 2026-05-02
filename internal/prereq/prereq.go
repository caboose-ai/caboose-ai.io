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
	results = append(results, c.checkOPLogin(ctx))
	return results
}

func (c *Checker) checkOPLogin(ctx context.Context) Result {
	name := "op signed in"
	out, err := c.Runner.Run(ctx, "op", "account", "list", "--format=json")
	if err != nil {
		return Result{Name: name, Err: fmt.Errorf("not signed in to 1Password: run `op signin` first")}
	}
	if len(out) < 3 {
		return Result{Name: name, Err: fmt.Errorf("no 1Password accounts configured: run `op account add`")}
	}
	return Result{Name: name, Found: true, Version: "authenticated"}
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
