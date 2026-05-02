package runner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
)

type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
	RunWithStdin(ctx context.Context, stdin io.Reader, name string, args ...string) ([]byte, error)
}

type LocalRunner struct{}

func NewLocalRunner() *LocalRunner {
	return &LocalRunner{}
}

func (r *LocalRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return r.RunWithStdin(ctx, nil, name, args...)
}

func (r *LocalRunner) RunWithStdin(ctx context.Context, stdin io.Reader, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if stdin != nil {
		cmd.Stdin = stdin
	}
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s: %w\n%s", name, err, stderr.String())
	}
	return stdout.Bytes(), nil
}
