package views

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/caboose-ai/caboose-ai.io/internal/install"
	"github.com/caboose-ai/caboose-ai.io/internal/tui/components"
	"github.com/caboose-ai/caboose-ai.io/internal/tui/styles"
)

type AllServicesConfiguredMsg struct {
	Results []install.ServiceResult
}

type ServiceConfiguredMsg struct {
	Index  int
	Result install.ServiceResult
}

type ServicesModel struct {
	lines   []components.StatusLineModel
	names   []string
	current int
	done    bool
}

func NewServices(names []string) ServicesModel {
	lines := make([]components.StatusLineModel, len(names))
	for i, name := range names {
		lines[i] = components.NewStatusLine(name)
	}
	return ServicesModel{lines: lines, names: names}
}

func (m ServicesModel) Init() tea.Cmd {
	if len(m.lines) > 0 {
		m.lines[0].Status = components.StatusRunning
		return m.lines[0].Init()
	}
	return nil
}

func (m ServicesModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ServiceConfiguredMsg:
		if msg.Result.Err != nil {
			m.lines[msg.Index].Status = components.StatusFailed
			m.lines[msg.Index].Message = msg.Result.Err.Error()
		} else if msg.Result.Result != nil {
			switch msg.Result.Result.Status.String() {
			case "skipped":
				m.lines[msg.Index].Status = components.StatusSkipped
			default:
				m.lines[msg.Index].Status = components.StatusDone
			}
			m.lines[msg.Index].Message = msg.Result.Result.Message
		}

		m.current = msg.Index + 1
		if m.current < len(m.lines) {
			m.lines[m.current].Status = components.StatusRunning
			return m, m.lines[m.current].Init()
		}
		m.done = true
		return m, nil
	}

	if m.current < len(m.lines) {
		var cmd tea.Cmd
		m.lines[m.current], cmd = m.lines[m.current].Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m ServicesModel) View() string {
	var lines []string
	for _, line := range m.lines {
		lines = append(lines, line.View())
	}
	return styles.ContentStyle.Render(
		"Configuring services:\n\n" + strings.Join(lines, "\n"),
	)
}
