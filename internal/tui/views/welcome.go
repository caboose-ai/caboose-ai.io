package views

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/caboose-ai/caboose-ai.io/internal/tui/styles"
)

type DomainConfirmedMsg struct{ Domain string }

type WelcomeModel struct {
	input textinput.Model
}

func NewWelcome(defaultDomain string) WelcomeModel {
	ti := textinput.New()
	ti.Placeholder = "example.com"
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = 40
	if defaultDomain != "" {
		ti.SetValue(defaultDomain)
	}
	return WelcomeModel{input: ti}
}

func (m WelcomeModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m WelcomeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if m.input.Value() != "" {
				return m, func() tea.Msg {
					return DomainConfirmedMsg{Domain: m.input.Value()}
				}
			}
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m WelcomeModel) View() string {
	return styles.ContentStyle.Render(
		"Enter your homelab domain:\n\n" +
			m.input.View() + "\n\n" +
			styles.DimStyle.Render("This will be used for auth.{domain}, git.{domain}, etc."),
	)
}
