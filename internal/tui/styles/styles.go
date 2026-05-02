package styles

import "github.com/charmbracelet/lipgloss"

var (
	ActivePhase   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	CompletePhase = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	PendingPhase  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	RunningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	SuccessStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	FailStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	SkipStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	DimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Padding(0, 1).
			BorderStyle(lipgloss.DoubleBorder()).
			BorderBottom(true).
			BorderForeground(lipgloss.Color("63"))

	ContentStyle = lipgloss.NewStyle().Padding(1, 2)

	ErrorBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("196")).
			Padding(1, 2)

	HelpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)
