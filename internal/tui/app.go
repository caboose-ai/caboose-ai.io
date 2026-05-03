package tui

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/caboose-ai/caboose-ai.io/internal/install"
	"github.com/caboose-ai/caboose-ai.io/internal/secrets"
	"github.com/caboose-ai/caboose-ai.io/internal/services"
	"github.com/caboose-ai/caboose-ai.io/internal/tui/components"
	"github.com/caboose-ai/caboose-ai.io/internal/tui/styles"
	"github.com/caboose-ai/caboose-ai.io/internal/tui/views"
)

// internal phase-transition messages
type vaultReadyMsg struct{}
type healthyMsg struct{}
type akReadyMsg struct{}
type providersReadyMsg struct{}
type outpostReadyMsg struct{}
type restartCompleteMsg struct{}
type socialReadyMsg struct{}
type turnstileReadyMsg struct{}
type adminInitDoneMsg struct{}
type captchaDoneMsg struct{}
type enrollmentDoneMsg struct{}
type forgejoDoneMsg struct{}
type progressMsg string

type AppModel struct {
	installer       *install.Installer
	stepper         components.StepperModel
	activeView      tea.Model
	width           int
	height          int
	quitting        bool
	promptValues    map[string]string
	bootstrapDefs   []secrets.SecretDef
	socialDefs      []secrets.SecretDef
	turnstileDefs   []secrets.SecretDef
	currentProgress string
}

func NewApp(installer *install.Installer) AppModel {
	stepper := components.NewStepper()
	welcome := views.NewWelcome(installer.Config.Domain)
	return AppModel{
		installer:     installer,
		stepper:       stepper,
		activeView:    welcome,
		promptValues:  make(map[string]string),
		bootstrapDefs: secrets.BootstrapSecrets(),
		socialDefs:    secrets.SocialSecrets(),
		turnstileDefs: secrets.TurnstileSecrets(),
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

	case progressMsg:
		m.currentProgress = string(msg)
		return m, nil

	// ── Domain confirmed → run prereqs ─────────────────────────────────────
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
		// Stepper: phase 0 = Secrets. Ensure vault first, then start secrets.
		m.stepper.Current = 0
		secretsView := views.NewSecrets()
		m.activeView = secretsView
		return m, tea.Batch(
			secretsView.Init(),
			m.runEnsureVault(),
		)

	case views.PrereqsFailedMsg:
		return m, tea.Quit

	// ── Vault ready → begin prompting / generating secrets ──────────────────
	case vaultReadyMsg:
		return m, m.continueSecretsPhase()

	// ── User submitted a prompted secret value ──────────────────────────────
	case views.SecretValueEnteredMsg:
		m.promptValues[msg.Key] = msg.Value
		return m, m.continueSecretsPhase()

	// ── Secrets generation errored ──────────────────────────────────────────
	case views.SecretsErrorMsg:
		// Forward to the active view so it can render the error state.
		var cmd tea.Cmd
		m.activeView, cmd = m.activeView.Update(msg)
		return m, cmd

	// ── Secrets complete → Resolve social credentials ──────────────────────
	case views.SecretsCompleteMsg:
		return m, m.runResolveSocialCredentials()

	// ── Social credentials ready → resolve Turnstile credentials ────────────
	case socialReadyMsg:
		return m, m.runResolveTurnstileCredentials()

	// ── Turnstile credentials ready → Compose phase ─────────────────────────
	case turnstileReadyMsg:
		m.stepper.Current = 1
		m.currentProgress = "Starting containers..."
		return m, m.runComposeAndHealth()

	// ── Compose + health done → init AK and rename admin ───────────────────
	case healthyMsg:
		m.currentProgress = "Renaming admin user to auth-admin..."
		return m, m.runInitAndRename()

	// ── AK init done → generate recovery link + store admin password ────────
	case adminInitDoneMsg:
		m.currentProgress = "Generating admin recovery link..."
		return m, m.runAdminRecoveryLink()

	// ── AK ready (recovery done) → setup captcha ────────────────────────────
	case akReadyMsg:
		m.currentProgress = "Creating Turnstile captcha stage..."
		return m, m.runSetupCaptcha()

	// ── Captcha done → setup inactive enrollment ────────────────────────────
	case captchaDoneMsg:
		m.currentProgress = "Configuring enrollment flow for inactive users..."
		return m, m.runSetupInactiveEnrollment()

	// ── Enrollment done → init Forgejo ──────────────────────────────────────
	case enrollmentDoneMsg:
		m.currentProgress = "Initializing Forgejo admin..."
		return m, m.runInitForgejo()

	// ── Forgejo done → provision OAuth2 providers ───────────────────────────
	case forgejoDoneMsg:
		m.currentProgress = "Provisioning OAuth2 providers..."
		return m, m.runProvisionProviders()

	// ── Providers ready → Provision outpost ─────────────────────────────────
	case providersReadyMsg:
		m.currentProgress = "Provisioning proxy providers..."
		return m, m.runProvisionOutpost()

	// ── Outpost ready → Configure phase ─────────────────────────────────────
	case outpostReadyMsg:
		m.currentProgress = ""
		m.stepper.Current = 2
		if err := m.installer.BuildServices(context.Background()); err != nil {
			return m, tea.Quit
		}
		if len(m.installer.Services) == 0 {
			summaryView := views.NewSummary(views.SummaryParams{
				Results:       m.installer.State.ServiceResults,
				Domain:        m.installer.State.Domain,
				AdminPassword: m.installer.State.AdminPassword,
				RecoveryLink:  m.installer.State.AdminRecoveryLink,
			})
			m.activeView = summaryView
			return m, summaryView.Init()
		}
		serviceNames := make([]string, len(m.installer.Services))
		for i, svc := range m.installer.Services {
			serviceNames[i] = svc.Name()
		}
		servicesView := views.NewServices(serviceNames)
		m.activeView = servicesView
		return m, tea.Batch(
			servicesView.Init(),
			m.configureServiceAtIndex(0),
		)

	// ── Per-service result received ──────────────────────────────────────────
	case views.ServiceConfiguredMsg:
		// Record the result in installer state (must happen on the main goroutine).
		m.installer.State.AddServiceResult(msg.Result.Name, msg.Result.Result, msg.Result.Err)
		// Forward to the services view for visual update.
		var viewCmd tea.Cmd
		m.activeView, viewCmd = m.activeView.Update(msg)
		// Schedule the next service or emit AllServicesConfiguredMsg.
		var nextCmd tea.Cmd
		if msg.Index+1 < len(m.installer.Services) {
			nextCmd = m.configureServiceAtIndex(msg.Index + 1)
		} else {
			results := m.installer.State.ServiceResults
			nextCmd = func() tea.Msg {
				return views.AllServicesConfiguredMsg{Results: results}
			}
		}
		return m, tea.Batch(viewCmd, nextCmd)

	// ── All services done → restart ─────────────────────────────────────────
	case views.AllServicesConfiguredMsg:
		return m, m.runRestartServices()

	// ── Restart done → summary ──────────────────────────────────────────────
	case restartCompleteMsg:
		summaryView := views.NewSummary(views.SummaryParams{
			Results:       m.installer.State.ServiceResults,
			Domain:        m.installer.State.Domain,
			AdminPassword: m.installer.State.AdminPassword,
			RecoveryLink:  m.installer.State.AdminRecoveryLink,
		})
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
	var progress string
	if m.currentProgress != "" {
		progress = "\n  " + styles.RunningStyle.Render("⠋ "+m.currentProgress)
	}
	content := m.activeView.View()
	help := styles.HelpStyle.Render("  q quit  enter confirm")

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		stepper+progress,
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

func (m AppModel) runEnsureVault() tea.Cmd {
	return func() tea.Msg {
		if err := m.installer.EnsureVault(context.Background()); err != nil {
			return views.SecretsErrorMsg{Err: fmt.Errorf("vault setup failed: %w", err)}
		}
		return vaultReadyMsg{}
	}
}

// continueSecretsPhase checks whether any Prompt:true secrets still need a
// value from the user. If so it emits SecretPromptNeededMsg for the first
// missing one. Once all prompts are satisfied it runs the batch generation.
func (m AppModel) continueSecretsPhase() tea.Cmd {
	for _, def := range m.bootstrapDefs {
		if !def.Prompt {
			continue
		}
		// Config file may already supply the value.
		if def.Key == "AUTHENTIK_BOOTSTRAP_EMAIL" && m.installer.Config.Email != "" {
			continue
		}
		if def.Key == "N8N_USER" && m.installer.Config.N8NUser != "" {
			continue
		}
		// Already entered interactively.
		if _, ok := m.promptValues[def.Key]; ok {
			continue
		}
		// Need to prompt.
		key := def.Key
		return func() tea.Msg { return views.SecretPromptNeededMsg{Key: key} }
	}
	for _, def := range m.socialDefs {
		if _, ok := m.promptValues[def.Key]; ok {
			continue
		}
		key := def.Key
		return func() tea.Msg { return views.SecretPromptNeededMsg{Key: key} }
	}
	for _, def := range m.turnstileDefs {
		if inst := m.installer; inst.Config.Turnstile.SiteKey != "" && def.Key == "TURNSTILE_SITE_KEY" {
			continue
		}
		if inst := m.installer; inst.Config.Turnstile.SecretKey != "" && def.Key == "TURNSTILE_SECRET_KEY" {
			continue
		}
		if _, ok := m.promptValues[def.Key]; ok {
			continue
		}
		key := def.Key
		return func() tea.Msg { return views.SecretPromptNeededMsg{Key: key} }
	}
	// All prompts satisfied — run batch generation.
	return m.runSecretsGeneration()
}

func (m AppModel) runSecretsGeneration() tea.Cmd {
	// Capture values before entering the goroutine.
	configEmail := m.installer.Config.Email
	configN8NUser := m.installer.Config.N8NUser
	promptVals := make(map[string]string, len(m.promptValues))
	for k, v := range m.promptValues {
		promptVals[k] = v
	}
	return func() tea.Msg {
		ctx := context.Background()
		err := m.installer.GenerateSecrets(ctx, func(key string) (string, error) {
			if key == "AUTHENTIK_BOOTSTRAP_EMAIL" && configEmail != "" {
				return configEmail, nil
			}
			if key == "N8N_USER" && configN8NUser != "" {
				return configN8NUser, nil
			}
			if val, ok := promptVals[key]; ok && val != "" {
				return val, nil
			}
			return "", fmt.Errorf("no value provided for required secret %s", key)
		}, nil)
		if err != nil {
			return views.SecretsErrorMsg{Err: err}
		}
		return views.SecretsCompleteMsg{}
	}
}

func (m AppModel) runComposeAndHealth() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		if err := m.installer.ComposeUp(ctx); err != nil {
			return views.SecretsErrorMsg{Err: fmt.Errorf("compose up failed: %w", err)}
		}
		// Drain the health channel; check for timeout.
		for status := range m.installer.WaitHealthy(ctx) {
			if status.Err == context.DeadlineExceeded {
				return views.SecretsErrorMsg{Err: fmt.Errorf("health check timed out waiting for %s to become ready", status.Name)}
			}
		}
		return healthyMsg{}
	}
}

func (m AppModel) runInitAndRename() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		token, err := m.installer.Secrets.Get(ctx, "AUTHENTIK_BOOTSTRAP_TOKEN")
		if err != nil {
			return views.SecretsErrorMsg{Err: fmt.Errorf("retrieving Authentik bootstrap token: %w", err)}
		}
		m.installer.InitAK(token)
		_ = m.installer.RenameAdmin(ctx)
		return adminInitDoneMsg{}
	}
}

func (m AppModel) runAdminRecoveryLink() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		link, err := m.installer.GenerateAdminRecoveryLink(ctx, nil)
		if err != nil {
			return views.SecretsErrorMsg{Err: fmt.Errorf("admin recovery link: %w", err)}
		}
		m.installer.State.AdminRecoveryLink = link
		adminPass, _ := m.installer.Secrets.Get(ctx, "AUTHENTIK_BOOTSTRAP_PASSWORD")
		m.installer.State.AdminPassword = adminPass
		return akReadyMsg{}
	}
}

func (m AppModel) runSetupCaptcha() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		if err := m.installer.SetupCaptcha(ctx, nil); err != nil {
			return views.SecretsErrorMsg{Err: fmt.Errorf("captcha setup: %w", err)}
		}
		return captchaDoneMsg{}
	}
}

func (m AppModel) runSetupInactiveEnrollment() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		if err := m.installer.SetupInactiveEnrollment(ctx, nil); err != nil {
			return views.SecretsErrorMsg{Err: fmt.Errorf("enrollment setup: %w", err)}
		}
		return enrollmentDoneMsg{}
	}
}

func (m AppModel) runInitForgejo() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		if err := m.installer.InitForgejo(ctx); err != nil {
			return views.SecretsErrorMsg{Err: fmt.Errorf("Forgejo admin init: %w", err)}
		}
		return forgejoDoneMsg{}
	}
}

func (m AppModel) configureServiceAtIndex(index int) tea.Cmd {
	return func() tea.Msg {
		svc := m.installer.Services[index]
		opts := services.ConfigureOpts{
			DryRun: m.installer.State.DryRun,
			Force:  m.installer.State.Force,
		}
		result, err := svc.Configure(context.Background(), opts)
		return views.ServiceConfiguredMsg{
			Index:  index,
			Result: install.ServiceResult{Name: svc.Name(), Result: result, Err: err},
		}
	}
}

func (m AppModel) runProvisionProviders() tea.Cmd {
	return func() tea.Msg {
		if err := m.installer.ProvisionProviders(context.Background(), nil); err != nil {
			return views.SecretsErrorMsg{Err: fmt.Errorf("provisioning providers: %w", err)}
		}
		return providersReadyMsg{}
	}
}

func (m AppModel) runProvisionOutpost() tea.Cmd {
	return func() tea.Msg {
		if err := m.installer.ProvisionOutpost(context.Background(), nil); err != nil {
			return views.SecretsErrorMsg{Err: fmt.Errorf("provisioning outpost: %w", err)}
		}
		return outpostReadyMsg{}
	}
}

func (m AppModel) runRestartServices() tea.Cmd {
	return func() tea.Msg {
		_ = m.installer.RestartServices(context.Background())
		return restartCompleteMsg{}
	}
}

func (m AppModel) runResolveTurnstileCredentials() tea.Cmd {
	promptVals := make(map[string]string, len(m.promptValues))
	for k, v := range m.promptValues {
		promptVals[k] = v
	}
	return func() tea.Msg {
		ctx := context.Background()
		err := m.installer.ResolveTurnstileCredentials(ctx, func(key string) (string, error) {
			if val, ok := promptVals[key]; ok && val != "" {
				return val, nil
			}
			return "", nil
		})
		if err != nil {
			return views.SecretsErrorMsg{Err: fmt.Errorf("resolving Turnstile credentials: %w", err)}
		}
		return turnstileReadyMsg{}
	}
}

func (m AppModel) runResolveSocialCredentials() tea.Cmd {
	promptVals := make(map[string]string, len(m.promptValues))
	for k, v := range m.promptValues {
		promptVals[k] = v
	}
	return func() tea.Msg {
		ctx := context.Background()
		err := m.installer.ResolveSocialCredentials(ctx, func(key string) (string, error) {
			if val, ok := promptVals[key]; ok && val != "" {
				return val, nil
			}
			return "", nil
		})
		if err != nil {
			return views.SecretsErrorMsg{Err: fmt.Errorf("resolving social credentials: %w", err)}
		}
		return socialReadyMsg{}
	}
}
