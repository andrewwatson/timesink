package tui

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	Quit key.Binding
	Help key.Binding
	Back key.Binding

	// Navigation
	Timer    key.Binding
	Entries  key.Binding
	Clients  key.Binding
	Invoices key.Binding
	Reports  key.Binding
	Settings key.Binding

	// Actions
	Select key.Binding
	New    key.Binding
	Edit   key.Binding
	Delete key.Binding

	// Movement
	Up    key.Binding
	Down  key.Binding
	Left  key.Binding
	Right key.Binding
}

var DefaultKeyMap = KeyMap{
	Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	Help:     key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Back:     key.NewBinding(key.WithKeys("esc", "backspace"), key.WithHelp("esc", "back")),
	Timer:    key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "timer")),
	Entries:  key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "entries")),
	Clients:  key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "clients")),
	Invoices: key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "invoices")),
	Reports:  key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "reports")),
	Settings: key.NewBinding(key.WithKeys(","), key.WithHelp(",", "settings")),
	Select:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
	New:      key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new")),
	Edit:     key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
	Delete:   key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
	Up:       key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:     key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	Left:     key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "left")),
	Right:    key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "right")),
}
