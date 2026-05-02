package services

import "context"

type Status int

const (
	StatusCreated Status = iota
	StatusUpdated
	StatusSkipped
	StatusAlreadyConfigured
	StatusDryRun
)

func (s Status) String() string {
	switch s {
	case StatusCreated:
		return "created"
	case StatusUpdated:
		return "updated"
	case StatusSkipped:
		return "skipped"
	case StatusAlreadyConfigured:
		return "already configured"
	case StatusDryRun:
		return "dry-run"
	default:
		return "unknown"
	}
}

type ConfigureOpts struct {
	DryRun bool
	Force  bool
}

type ConfigureResult struct {
	Status          Status
	Message         string
	RestartRequired bool
	Services        []string
}

type ServiceConfigurator interface {
	Name() string
	Slug() string
	CheckConfigured(ctx context.Context) (bool, error)
	Configure(ctx context.Context, opts ConfigureOpts) (*ConfigureResult, error)
}
