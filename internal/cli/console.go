package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	clog "github.com/charmbracelet/log"
)

type Console struct {
	logger *clog.Logger
	phase  int
}

var (
	consoleTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	consoleDimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	consolePhaseStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	consoleRunStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	consoleOKStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	consoleWarnStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	consoleFailStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	consoleURLStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("45"))
)

func NewConsole() *Console {
	return &Console{
		logger: clog.NewWithOptions(os.Stderr, clog.Options{ReportTimestamp: true}),
	}
}

func (c *Console) Banner(title, subtitle string) {
	line := strings.Repeat("═", lipgloss.Width(title)+2)
	c.logger.Info(consoleTitleStyle.Render(title))
	c.logger.Info(consoleDimStyle.Render(line))
	if subtitle != "" {
		c.logger.Info(consoleDimStyle.Render(subtitle))
	}
}

func (c *Console) Phase(name string) {
	c.phase++
	c.logger.Info(consolePhaseStyle.Render(fmt.Sprintf("%02d %s", c.phase, name)))
}

func (c *Console) Info(msg string, attrs ...any) {
	c.logger.Info(msg, attrs...)
}

func (c *Console) Run(msg string, attrs ...any) {
	c.logger.Info(consoleRunStyle.Render("●")+" "+msg, attrs...)
}

func (c *Console) Success(msg string, attrs ...any) {
	c.logger.Info(consoleOKStyle.Render("✓")+" "+msg, attrs...)
}

func (c *Console) Warn(msg string, attrs ...any) {
	c.logger.Warn(consoleWarnStyle.Render("!")+" "+msg, attrs...)
}

func (c *Console) Error(msg string, attrs ...any) {
	c.logger.Error(consoleFailStyle.Render("✗")+" "+msg, attrs...)
}

func (c *Console) Skip(msg string, attrs ...any) {
	c.logger.Warn(consoleWarnStyle.Render("-")+" "+msg, attrs...)
}

func (c *Console) Link(label, url string) {
	c.logger.Info(consoleOKStyle.Render("→") + " " + label + " " + consoleURLStyle.Render(url))
}
