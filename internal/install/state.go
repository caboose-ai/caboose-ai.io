package install

import "github.com/caboose-ai/caboose-ai.io/internal/services"

type Phase int

const (
	PhaseCheckPrereq Phase = iota
	PhasePromptDomain
	PhaseEnsureVault
	PhaseGenSecrets
	PhaseWriteEnv
	PhaseComposeUp
	PhaseWaitHealthy
	PhaseRenameAdmin
	PhaseSetupForgejo
	PhaseSetupWoodpecker
	PhaseSetupPortainer
	PhaseSetupGrafana
	PhaseSetupOpenWebUI
	PhaseSetupMattermost
	PhaseSetupSocial
	PhaseRestartServices
	PhaseFinalHealth
	PhaseSummary
	PhaseDone
)

func (p Phase) String() string {
	names := [...]string{
		"Check Prerequisites", "Prompt Domain", "Ensure Vault",
		"Generate Secrets", "Write .env", "Compose Up",
		"Wait for Health", "Rename Admin",
		"Setup Forgejo", "Setup Woodpecker", "Setup Portainer",
		"Setup Grafana", "Setup Open WebUI", "Setup Mattermost",
		"Setup Social Login", "Restart Services", "Final Health Check",
		"Summary", "Done",
	}
	if int(p) < len(names) {
		return names[p]
	}
	return "unknown"
}

func (p Phase) Group() int {
	switch {
	case p <= PhaseWriteEnv:
		return 1
	case p <= PhaseRenameAdmin:
		return 2
	default:
		return 3
	}
}

type ServiceResult struct {
	Name   string
	Result *services.ConfigureResult
	Err    error
}

type State struct {
	Phase          Phase
	Domain         string
	Email          string
	ComposeDir     string
	DryRun         bool
	Force          bool

	ServiceResults []ServiceResult
	RestartNeeded  []string
	Errors         []error
}

func NewState() *State {
	return &State{Phase: PhaseCheckPrereq}
}

func (s *State) Advance() {
	if s.Phase < PhaseDone {
		s.Phase++
	}
}

func (s *State) AddServiceResult(name string, result *services.ConfigureResult, err error) {
	s.ServiceResults = append(s.ServiceResults, ServiceResult{Name: name, Result: result, Err: err})
	if result != nil && result.RestartRequired {
		s.RestartNeeded = append(s.RestartNeeded, result.Services...)
	}
	if err != nil {
		s.Errors = append(s.Errors, err)
	}
}
