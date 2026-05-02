package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/caboose-ai/caboose-ai.io/internal/prereq"
	"github.com/caboose-ai/caboose-ai.io/internal/tui/styles"
)

type PrereqsPassedMsg struct{}
type PrereqsFailedMsg struct{ Missing []string }
type PrereqsResultMsg struct{ Results []prereq.Result }

type PrereqsModel struct {
	spinner spinner.Model
	results []prereq.Result
	done    bool
}

func NewPrereqs() PrereqsModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	return PrereqsModel{spinner: s}
}

func (m PrereqsModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m PrereqsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case PrereqsResultMsg:
		m.results = msg.Results
		m.done = true
		if prereq.AllPassed(msg.Results) {
			return m, func() tea.Msg { return PrereqsPassedMsg{} }
		}
		var missing []string
		for _, r := range msg.Results {
			if !r.Found {
				missing = append(missing, r.Name)
			}
		}
		return m, func() tea.Msg { return PrereqsFailedMsg{Missing: missing} }
	}

	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m PrereqsModel) View() string {
	if !m.done {
		return styles.ContentStyle.Render(
			m.spinner.View() + " Checking prerequisites...",
		)
	}

	var lines []string
	for _, r := range m.results {
		if r.Found {
			lines = append(lines, styles.SuccessStyle.Render("  ✓ ")+r.Name)
		} else {
			lines = append(lines, styles.FailStyle.Render("  ✗ ")+r.Name+
				styles.DimStyle.Render(fmt.Sprintf(" (%v)", r.Err)))
		}
	}
	return styles.ContentStyle.Render(
		"Prerequisites:\n\n" + strings.Join(lines, "\n"),
	)
}
