package tui

import (
	"fmt"

	"github.com/andy/timesink/internal/app"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Screen represents the current active screen
type Screen int

const (
	ScreenDashboard Screen = iota
	ScreenTimer
	ScreenEntries
	ScreenClients
	ScreenInvoices
	ScreenReports
)

// String returns the screen name
func (s Screen) String() string {
	switch s {
	case ScreenDashboard:
		return "Dashboard"
	case ScreenTimer:
		return "Timer"
	case ScreenEntries:
		return "Time Entries"
	case ScreenClients:
		return "Clients"
	case ScreenInvoices:
		return "Invoices"
	case ScreenReports:
		return "Reports"
	default:
		return "Unknown"
	}
}

// Model is the root Bubble Tea model
type Model struct {
	app           *app.App
	currentScreen Screen
	width         int
	height        int

	// Screen models (lazy initialized)
	dashboard tea.Model
	timer     tea.Model
	entries   tea.Model
	clients   tea.Model
	invoices  tea.Model
	reports   tea.Model

	// Error state
	err error
}

// New creates a new root model
func New(a *app.App) Model {
	return Model{
		app:           a,
		currentScreen: ScreenDashboard,
		width:         0,
		height:        0,
	}
}

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	// Start with dashboard initialization
	return nil
}

// Update implements tea.Model - routes keys to screens
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		// Global key handlers
		switch {
		case key.Matches(msg, DefaultKeyMap.Quit):
			return m, tea.Quit

		case key.Matches(msg, DefaultKeyMap.Timer):
			m.currentScreen = ScreenTimer
			return m, nil

		case key.Matches(msg, DefaultKeyMap.Entries):
			m.currentScreen = ScreenEntries
			return m, nil

		case key.Matches(msg, DefaultKeyMap.Clients):
			m.currentScreen = ScreenClients
			return m, nil

		case key.Matches(msg, DefaultKeyMap.Invoices):
			m.currentScreen = ScreenInvoices
			return m, nil

		case key.Matches(msg, DefaultKeyMap.Reports):
			m.currentScreen = ScreenReports
			return m, nil
		}

	case SwitchScreenMsg:
		m.currentScreen = msg.Screen
		return m, nil

	case ErrorMsg:
		m.err = msg.Err
		return m, nil
	}

	// Route message to current screen
	var cmd tea.Cmd
	switch m.currentScreen {
	case ScreenDashboard:
		if m.dashboard != nil {
			m.dashboard, cmd = m.dashboard.Update(msg)
		}
	case ScreenTimer:
		if m.timer != nil {
			m.timer, cmd = m.timer.Update(msg)
		}
	case ScreenEntries:
		if m.entries != nil {
			m.entries, cmd = m.entries.Update(msg)
		}
	case ScreenClients:
		if m.clients != nil {
			m.clients, cmd = m.clients.Update(msg)
		}
	case ScreenInvoices:
		if m.invoices != nil {
			m.invoices, cmd = m.invoices.Update(msg)
		}
	case ScreenReports:
		if m.reports != nil {
			m.reports, cmd = m.reports.Update(msg)
		}
	}

	return m, cmd
}

// View implements tea.Model - renders header + current screen + footer
func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Header
	header := headerStyle.Render(fmt.Sprintf("timesink - %s", m.currentScreen.String()))

	// Footer with navigation keys
	footer := footerStyle.Render("[T]imer  [E]ntries  [C]lients  [I]nvoices  [R]eports  [Q]uit")

	// Current screen content
	var content string
	switch m.currentScreen {
	case ScreenDashboard:
		if m.dashboard != nil {
			content = m.dashboard.View()
		} else {
			content = "Dashboard - Coming Soon"
		}
	case ScreenTimer:
		if m.timer != nil {
			content = m.timer.View()
		} else {
			content = "Timer - Coming Soon"
		}
	case ScreenEntries:
		if m.entries != nil {
			content = m.entries.View()
		} else {
			content = "Time Entries - Coming Soon"
		}
	case ScreenClients:
		if m.clients != nil {
			content = m.clients.View()
		} else {
			content = "Clients - Coming Soon"
		}
	case ScreenInvoices:
		if m.invoices != nil {
			content = m.invoices.View()
		} else {
			content = "Invoices - Coming Soon"
		}
	case ScreenReports:
		if m.reports != nil {
			content = m.reports.View()
		} else {
			content = "Reports - Coming Soon"
		}
	}

	// Error display
	errorDisplay := ""
	if m.err != nil {
		errorDisplay = lipgloss.NewStyle().
			Foreground(errorColor).
			Render(fmt.Sprintf("\nError: %s", m.err.Error()))
	}

	return fmt.Sprintf("%s\n\n%s%s\n\n%s", header, content, errorDisplay, footer)
}

// Run starts the TUI
func Run(a *app.App) error {
	p := tea.NewProgram(New(a), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
