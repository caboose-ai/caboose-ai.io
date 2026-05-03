package orchestrator

import (
	"context"

	"github.com/caboose-ai/caboose-ai.io/internal/docker"
)

type ComposeBackend struct{ compose *docker.ComposeClient }

func NewComposeBackend(compose *docker.ComposeClient) *ComposeBackend { return &ComposeBackend{compose: compose} }
func (b *ComposeBackend) Name() string                                { return "compose" }
func (b *ComposeBackend) Apply(ctx context.Context) error {
	_, err := b.compose.Up(ctx)
	return err
}
func (b *ComposeBackend) Teardown(ctx context.Context) error {
	_, err := b.compose.DownWithVolumes(ctx)
	return err
}
