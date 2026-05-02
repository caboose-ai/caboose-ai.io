package components

import (
	"fmt"
	"strings"

	"github.com/caboose-ai/caboose-ai.io/internal/tui/styles"
)

type StepperModel struct {
	Steps   []string
	Current int
}

func NewStepper() StepperModel {
	return StepperModel{
		Steps: []string{"Secrets", "Compose", "Configure"},
	}
}

func (m StepperModel) View() string {
	var parts []string
	for i, step := range m.Steps {
		label := fmt.Sprintf("%d. %s", i+1, step)
		switch {
		case i < m.Current:
			parts = append(parts, styles.CompletePhase.Render("✓ "+label))
		case i == m.Current:
			parts = append(parts, styles.ActivePhase.Render("● "+label))
		default:
			parts = append(parts, styles.PendingPhase.Render("○ "+label))
		}
	}
	return "  " + strings.Join(parts, "  ──  ")
}
