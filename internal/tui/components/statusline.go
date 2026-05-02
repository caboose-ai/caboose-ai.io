package components

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/caboose-ai/caboose-ai.io/internal/tui/styles"
)

type ServiceStatus int

const (
	StatusPending ServiceStatus = iota
	StatusRunning
	StatusDone
	StatusFailed
	StatusSkipped
)

type StatusLineModel struct {
	Name    string
	Status  ServiceStatus
	Message string
	spinner spinner.Model
}

func NewStatusLine(name string) StatusLineModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	return StatusLineModel{Name: name, spinner: s}
}

func (m StatusLineModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m StatusLineModel) Update(msg tea.Msg) (StatusLineModel, tea.Cmd) {
	if m.Status == StatusRunning {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m StatusLineModel) View() string {
	var icon, detail string
	switch m.Status {
	case StatusPending:
		icon = styles.DimStyle.Render("○")
		detail = styles.DimStyle.Render("pending")
	case StatusRunning:
		icon = m.spinner.View()
		detail = styles.RunningStyle.Render("configuring...")
	case StatusDone:
		icon = styles.SuccessStyle.Render("✓")
		detail = styles.SuccessStyle.Render(m.Message)
	case StatusFailed:
		icon = styles.FailStyle.Render("✗")
		detail = styles.FailStyle.Render(m.Message)
	case StatusSkipped:
		icon = styles.SkipStyle.Render("–")
		detail = styles.SkipStyle.Render(m.Message)
	}

	return fmt.Sprintf("  %s %-25s %s", icon, m.Name, detail)
}
