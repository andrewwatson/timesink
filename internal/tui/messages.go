package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// SwitchScreenMsg requests a screen change
type SwitchScreenMsg struct {
	Screen Screen
}

// RefreshDataMsg requests data refresh
type RefreshDataMsg struct{}

// ErrorMsg carries error information
type ErrorMsg struct {
	Err error
}

// TimerTickMsg is sent every second when timer is running
type TimerTickMsg struct{}

// tickTimer returns a command that sends TimerTickMsg every second
func tickTimer() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return TimerTickMsg{}
	})
}
