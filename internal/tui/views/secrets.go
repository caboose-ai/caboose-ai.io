package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/caboose-ai/caboose-ai.io/internal/secrets"
	"github.com/caboose-ai/caboose-ai.io/internal/tui/styles"
)

type SecretsCompleteMsg struct{}

// SecretGeneratedMsg is emitted by the app for each auto-generated secret.
type SecretGeneratedMsg struct {
	Key string
	Err error
}

// SecretPromptNeededMsg is emitted by the app when a prompted secret has no config value.
type SecretPromptNeededMsg struct{ Key string }

// SecretValueEnteredMsg is emitted by SecretsModel when the user submits a prompt.
type SecretValueEnteredMsg struct {
	Key   string
	Value string
}

// SecretsErrorMsg is emitted by the app when secret generation fails.
type SecretsErrorMsg struct{ Err error }

type SecretsModel struct {
	defs      []secrets.SecretDef
	defByKey  map[string]secrets.SecretDef
	statuses  map[string]string // "done", "generating", "prompting", "error"
	errors    map[string]error
	spinner   spinner.Model
	input     textinput.Model
	prompting string
	done      bool
	hasError  bool
	err       error
}

func NewSecrets() SecretsModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	ti := textinput.New()
	ti.CharLimit = 100
	ti.Width = 40
	defs := append(secrets.BootstrapSecrets(), secrets.SocialSecrets()...)
	defByKey := make(map[string]secrets.SecretDef, len(defs))
	for _, def := range defs {
		defByKey[def.Key] = def
	}
	return SecretsModel{
		defs:     defs,
		defByKey: defByKey,
		statuses: make(map[string]string),
		errors:   make(map[string]error),
		spinner:  s,
		input:    ti,
	}
}

func (m SecretsModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m SecretsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case SecretGeneratedMsg:
		if msg.Err != nil {
			m.statuses[msg.Key] = "error"
			m.errors[msg.Key] = msg.Err
		} else {
			m.statuses[msg.Key] = "done"
		}
		return m, nil

	case SecretPromptNeededMsg:
		m.prompting = msg.Key
		m.statuses[msg.Key] = "prompting"
		m.input.Placeholder = msg.Key
		if def, ok := m.defByKey[msg.Key]; ok && def.Optional {
			m.input.Placeholder = msg.Key + " (optional, Enter to skip)"
		}
		m.input.Focus()
		return m, textinput.Blink

	case SecretsErrorMsg:
		m.hasError = true
		m.err = msg.Err
		m.done = true
		return m, nil

	case tea.KeyMsg:
		if m.prompting != "" && msg.String() == "enter" {
			val := m.input.Value()
			def, ok := m.defByKey[m.prompting]
			if val != "" || (ok && def.Optional) {
				key := m.prompting
				m.statuses[key] = "done"
				m.prompting = ""
				m.input.SetValue("")
				m.input.Blur()
				// Notify the app so it can continue the secrets phase.
				return m, func() tea.Msg { return SecretValueEnteredMsg{Key: key, Value: val} }
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	if m.prompting != "" {
		m.input, cmd = m.input.Update(msg)
	} else {
		m.spinner, cmd = m.spinner.Update(msg)
	}
	return m, cmd
}

func (m SecretsModel) View() string {
	if m.hasError {
		return styles.ContentStyle.Render(
			styles.FailStyle.Render("✗ Secret generation failed:\n\n  ") +
				m.err.Error() + "\n\n" +
				styles.DimStyle.Render("Press q to quit"),
		)
	}

	var lines []string
	for _, def := range m.defs {
		status := m.statuses[def.Key]
		switch status {
		case "done":
			lines = append(lines, styles.SuccessStyle.Render("  ✓ ")+def.Key)
		case "generating":
			lines = append(lines, fmt.Sprintf("  %s %s", m.spinner.View(), def.Key))
		case "prompting":
			lines = append(lines, styles.RunningStyle.Render("  ? ")+def.Key)
		case "error":
			lines = append(lines, styles.FailStyle.Render("  ✗ ")+def.Key+
				styles.DimStyle.Render(fmt.Sprintf(" (%v)", m.errors[def.Key])))
		default:
			lines = append(lines, styles.DimStyle.Render("  ○ ")+styles.DimStyle.Render(def.Key))
		}
	}

	view := "Generating secrets:\n\n" + strings.Join(lines, "\n")
	if m.prompting != "" {
		view += "\n\n  Enter " + m.prompting + ":\n  " + m.input.View()
	}
	return styles.ContentStyle.Render(view)
}
