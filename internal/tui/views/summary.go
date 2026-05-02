package views

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/caboose-ai/caboose-ai.io/internal/install"
	"github.com/caboose-ai/caboose-ai.io/internal/tui/styles"
)

type SummaryModel struct {
	results []install.ServiceResult
	domain  string
}

func NewSummary(results []install.ServiceResult, domain string) SummaryModel {
	return SummaryModel{results: results, domain: domain}
}

func (m SummaryModel) Init() tea.Cmd { return nil }

func (m SummaryModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "enter", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m SummaryModel) View() string {
	var lines []string
	lines = append(lines, "Install complete!\n")

	for _, r := range m.results {
		if r.Err != nil {
			lines = append(lines, styles.FailStyle.Render(fmt.Sprintf("  ✗ %-25s %s", r.Name, r.Err)))
		} else if r.Result != nil {
			switch r.Result.Status.String() {
			case "skipped":
				lines = append(lines, styles.SkipStyle.Render(fmt.Sprintf("  – %-25s %s", r.Name, r.Result.Message)))
			default:
				lines = append(lines, styles.SuccessStyle.Render(fmt.Sprintf("  ✓ %-25s %s", r.Name, r.Result.Message)))
			}
		}
	}

	lines = append(lines, "")
	lines = append(lines, "Service URLs:")
	lines = append(lines, fmt.Sprintf("  Authentik:  https://auth.%s", m.domain))
	lines = append(lines, fmt.Sprintf("  Forgejo:    https://git.%s", m.domain))
	lines = append(lines, fmt.Sprintf("  Portainer:  https://docker.%s", m.domain))
	lines = append(lines, fmt.Sprintf("  Grafana:    https://grafana.%s", m.domain))
	lines = append(lines, fmt.Sprintf("  Woodpecker: https://ci.%s", m.domain))
	lines = append(lines, "")
	lines = append(lines, styles.DimStyle.Render("Press q or enter to exit"))

	return styles.ContentStyle.Render(strings.Join(lines, "\n"))
}
