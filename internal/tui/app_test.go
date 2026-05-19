package tui

import (
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
	"github.com/caboose-ai/caboose-ai.io/internal/install"
	"github.com/caboose-ai/caboose-ai.io/internal/tui/views"
)

func TestRestartCompleteShowsSummaryWithoutBootstrappingPaperclip(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Domain = "example.test"
	inst := install.New(cfg, nil, nil, nil)
	inst.State.Domain = cfg.Domain

	model := NewApp(inst)
	updated, cmd := model.Update(restartCompleteMsg{})
	if cmd != nil {
		t.Fatal("restartCompleteMsg returned a command; want immediate summary without Paperclip bootstrap")
	}

	app, ok := updated.(AppModel)
	if !ok {
		t.Fatalf("updated model = %T, want AppModel", updated)
	}
	if app.currentProgress != "" {
		t.Fatalf("currentProgress = %q, want empty", app.currentProgress)
	}
	if _, ok := app.activeView.(views.SummaryModel); !ok {
		t.Fatalf("activeView = %T, want views.SummaryModel", app.activeView)
	}
}
