package orchestrator

import "context"

type Backend interface {
	Name() string
	Apply(ctx context.Context) error
	Teardown(ctx context.Context) error
}
