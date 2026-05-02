package tui

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	Quit    key.Binding
	Enter   key.Binding
	Retry   key.Binding
	Skip    key.Binding
}

var Keys = KeyMap{
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "confirm"),
	),
	Retry: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "retry"),
	),
	Skip: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "skip"),
	),
}
