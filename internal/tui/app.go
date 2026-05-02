package tui

import (
	"context"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/caboose-ai/caboose-ai.io/internal/install"
	"github.com/caboose-ai/caboose-ai.io/internal/tui/components"
	"github.com/caboose-ai/caboose-ai.io/internal/tui/styles"
	"github.com/caboose-ai/caboose-ai.io/internal/tui/views"
)

type AppModel struct {
	installer  *install.Installer
	stepper    components.StepperModel
	activeView tea.Model
	width      int
	height     int
	quitting   bool
}

func NewApp(installer *install.Installer) AppModel {
	stepper := components.NewStepper()
	welcome := views.NewWelcome(installer.Config.Domain)
	return AppModel{
		installer:  installer,
		stepper:    stepper,
		activeView: welcome,
	}
}

func (m AppModel) Init() tea.Cmd {
	return m.activeView.Init()
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		if key.Matches(msg, Keys.Quit) {
			m.quitting = true
			return m, tea.Quit
		}

	case views.DomainConfirmedMsg:
		m.installer.State.Domain = msg.Domain
		m.installer.Config.Domain = msg.Domain
		prereqView := views.NewPrereqs()
		m.activeView = prereqView
		return m, tea.Batch(
			prereqView.Init(),
			m.checkPrereqs(),
		)

	case views.PrereqsPassedMsg:
		m.stepper.Current = 0
		secretsView := views.NewSecrets()
		m.activeView = secretsView
		return m, tea.Batch(
			secretsView.Init(),
			m.runSecretsGeneration(),
		)

	case views.PrereqsFailedMsg:
		return m, tea.Quit

	case views.SecretsCompleteMsg:
		m.stepper.Current = 1
		serviceNames := make([]string, len(m.installer.Services))
		for i, svc := range m.installer.Services {
			serviceNames[i] = svc.Name()
		}
		servicesView := views.NewServices(serviceNames)
		m.activeView = servicesView
		return m, tea.Batch(
			servicesView.Init(),
			m.runServiceConfiguration(),
		)

	case views.AllServicesConfiguredMsg:
		m.stepper.Current = 2
		summaryView := views.NewSummary(msg.Results, m.installer.State.Domain)
		m.activeView = summaryView
		return m, summaryView.Init()
	}

	var cmd tea.Cmd
	m.activeView, cmd = m.activeView.Update(msg)
	return m, cmd
}

func (m AppModel) View() string {
	if m.quitting {
		return ""
	}

	header := styles.HeaderStyle.Render("Homelab SSO Installer")
	stepper := m.stepper.View()
	content := m.activeView.View()
	help := styles.HelpStyle.Render("  q quit  enter confirm")

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		stepper,
		content,
		help,
	)
}

func (m AppModel) checkPrereqs() tea.Cmd {
	return func() tea.Msg {
		results, _ := m.installer.CheckPrereqs(context.Background())
		return views.PrereqsResultMsg{Results: results}
	}
}

func (m AppModel) runSecretsGeneration() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		err := m.installer.GenerateSecrets(ctx, func(key string) (string, error) {
			if key == "AUTHENTIK_BOOTSTRAP_EMAIL" && m.installer.Config.Email != "" {
				return m.installer.Config.Email, nil
			}
			if key == "N8N_USER" && m.installer.Config.N8NUser != "" {
				return m.installer.Config.N8NUser, nil
			}
			return "", nil
		})
		if err != nil {
			return views.PrereqsFailedMsg{}
		}
		return views.SecretsCompleteMsg{}
	}
}

func (m AppModel) runServiceConfiguration() tea.Cmd {
	return func() tea.Msg {
		results := m.installer.ConfigureServices(context.Background())
		return views.AllServicesConfiguredMsg{Results: results}
	}
}
