package views

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/caboose-ai/caboose-ai.io/internal/install"
	"github.com/caboose-ai/caboose-ai.io/internal/tui/styles"
)

// SummaryParams holds everything the summary view needs to render.
type SummaryParams struct {
	Results       []install.ServiceResult
	Domain        string
	AdminPassword string
	RecoveryLink  string
}

type SummaryModel struct {
	results       []install.ServiceResult
	domain        string
	adminPassword string
	recoveryLink  string
}

func NewSummary(params SummaryParams) SummaryModel {
	return SummaryModel{
		results:       params.Results,
		domain:        params.Domain,
		adminPassword: params.AdminPassword,
		recoveryLink:  params.RecoveryLink,
	}
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

	if m.adminPassword != "" {
		border := styles.DimStyle.Render("──────────────────────────────────────")
		lines = append(lines, "")
		lines = append(lines, border)
		lines = append(lines, "  Auth Admin Credentials")
		lines = append(lines, border)
		lines = append(lines, "  Username:  auth-admin")
		lines = append(lines, fmt.Sprintf("  Password:  %s", m.adminPassword))
		if m.recoveryLink != "" {
			lines = append(lines, "")
			lines = append(lines, "  One-time setup link (set up TOTP):")
			lines = append(lines, fmt.Sprintf("    %s", m.recoveryLink))
		}
		lines = append(lines, "")
		lines = append(lines, styles.DimStyle.Render("  Stored in 1Password:"))
		lines = append(lines, styles.DimStyle.Render(`    op read "op://Homelab/AUTHENTIK_BOOTSTRAP_PASSWORD/password"`))
		lines = append(lines, border)
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
