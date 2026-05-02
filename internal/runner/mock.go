package runner

import (
	"context"
	"fmt"
	"io"
	"strings"
)

type Call struct {
	Name string
	Args []string
}

type MockRunner struct {
	Calls    []Call
	Handlers map[string]func(args []string) ([]byte, error)
}

func NewMockRunner() *MockRunner {
	return &MockRunner{
		Handlers: make(map[string]func(args []string) ([]byte, error)),
	}
}

func (m *MockRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return m.RunWithStdin(ctx, nil, name, args...)
}

func (m *MockRunner) RunWithStdin(_ context.Context, _ io.Reader, name string, args ...string) ([]byte, error) {
	m.Calls = append(m.Calls, Call{Name: name, Args: args})

	key := name + " " + strings.Join(args, " ")
	for pattern, handler := range m.Handlers {
		if strings.HasPrefix(key, pattern) {
			return handler(args)
		}
	}
	return nil, fmt.Errorf("no mock handler for: %s", key)
}

func (m *MockRunner) On(pattern string, output []byte, err error) {
	m.Handlers[pattern] = func(_ []string) ([]byte, error) {
		return output, err
	}
}

func (m *MockRunner) OnFunc(pattern string, fn func(args []string) ([]byte, error)) {
	m.Handlers[pattern] = fn
}
