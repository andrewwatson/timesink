package tui

import (
	"context"
	"fmt"
	"strings"

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
	ScreenSettings
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
	case ScreenSettings:
		return "Settings"
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
	settings  tea.Model

	// First-run state
	checkedFirstRun bool

	// Error state
	err     error
	quitMsg string // shown when quit is blocked
}

// New creates a new root model
func New(a *app.App) Model {
	dashboard := NewDashboardModel(a)
	return Model{
		app:           a,
		currentScreen: ScreenDashboard,
		dashboard:     dashboard,
	}
}

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		m.checkFirstRun(),
	}
	if m.dashboard != nil {
		cmds = append(cmds, m.dashboard.Init())
	}
	return tea.Batch(cmds...)
}

// checkFirstRun checks if any clients exist in the database
func (m *Model) checkFirstRun() tea.Cmd {
	return func() tea.Msg {
		clients, err := m.app.ClientRepo.List(context.Background(), false)
		if err != nil {
			return firstRunCheckMsg{hasClients: true} // assume yes on error
		}
		return firstRunCheckMsg{hasClients: len(clients) > 0}
	}
}

// initScreen lazy-initializes a screen on first visit,
// and sends a RefreshDataMsg on subsequent visits so screens reload data.
func (m *Model) initScreen(screen Screen) tea.Cmd {
	switch screen {
	case ScreenDashboard:
		if m.dashboard == nil {
			m.dashboard = NewDashboardModel(m.app)
			return m.dashboard.Init()
		}
		return func() tea.Msg { return RefreshDataMsg{} }
	case ScreenTimer:
		if m.timer == nil {
			m.timer = NewTimerModel(m.app)
			return m.timer.Init()
		}
		return func() tea.Msg { return RefreshDataMsg{} }
	case ScreenEntries:
		if m.entries == nil {
			m.entries = NewEntriesModel(m.app)
			return m.entries.Init()
		}
		return func() tea.Msg { return RefreshDataMsg{} }
	case ScreenClients:
		if m.clients == nil {
			m.clients = NewClientsModel(m.app)
			return m.clients.Init()
		}
		return func() tea.Msg { return RefreshDataMsg{} }
	case ScreenInvoices:
		if m.invoices == nil {
			m.invoices = NewInvoicesModel(m.app)
			return m.invoices.Init()
		}
		return func() tea.Msg { return RefreshDataMsg{} }
	case ScreenReports:
		if m.reports == nil {
			m.reports = NewReportsModel(m.app)
			return m.reports.Init()
		}
		return func() tea.Msg { return RefreshDataMsg{} }
	case ScreenSettings:
		if m.settings == nil {
			m.settings = NewSettingsModel(m.app)
			return m.settings.Init()
		}
		return func() tea.Msg { return RefreshDataMsg{} }
	}
	return nil
}

// InputCapturer is implemented by screens that capture keyboard input (e.g. text forms).
// When active, global navigation keys (T, E, C, I, R, Q) are suppressed.
type InputCapturer interface {
	IsCapturingInput() bool
}

// activeScreenCapturingInput returns true if the current screen is capturing text input
func (m *Model) activeScreenCapturingInput() bool {
	var screen tea.Model
	switch m.currentScreen {
	case ScreenDashboard:
		screen = m.dashboard
	case ScreenTimer:
		screen = m.timer
	case ScreenEntries:
		screen = m.entries
	case ScreenClients:
		screen = m.clients
	case ScreenInvoices:
		screen = m.invoices
	case ScreenReports:
		screen = m.reports
	case ScreenSettings:
		screen = m.settings
	}
	if ic, ok := screen.(InputCapturer); ok {
		return ic.IsCapturingInput()
	}
	return false
}

// Update implements tea.Model - routes keys to screens
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		// Clear quit warning on any keypress
		m.quitMsg = ""

		// Skip global navigation when a screen is capturing text input
		if !m.activeScreenCapturingInput() {
			// Global key handlers (screen navigation)
			switch {
			case key.Matches(msg, DefaultKeyMap.Quit):
				t, _ := m.app.TimerService.GetActiveTimer(context.Background())
				if t != nil {
					m.quitMsg = "Timer is running. Stop or discard it before quitting."
					return m, nil
				}
				return m, tea.Quit

			case key.Matches(msg, DefaultKeyMap.Timer):
				m.currentScreen = ScreenTimer
				cmd := m.initScreen(ScreenTimer)
				return m, cmd

			case key.Matches(msg, DefaultKeyMap.Entries):
				m.currentScreen = ScreenEntries
				cmd := m.initScreen(ScreenEntries)
				return m, cmd

			case key.Matches(msg, DefaultKeyMap.Clients):
				m.currentScreen = ScreenClients
				cmd := m.initScreen(ScreenClients)
				return m, cmd

			case key.Matches(msg, DefaultKeyMap.Invoices):
				m.currentScreen = ScreenInvoices
				cmd := m.initScreen(ScreenInvoices)
				return m, cmd

			case key.Matches(msg, DefaultKeyMap.Reports):
				m.currentScreen = ScreenReports
				cmd := m.initScreen(ScreenReports)
				return m, cmd

			case key.Matches(msg, DefaultKeyMap.Settings):
				m.currentScreen = ScreenSettings
				cmd := m.initScreen(ScreenSettings)
				return m, cmd
			}
		}

	case firstRunCheckMsg:
		if !m.checkedFirstRun && !msg.hasClients {
			m.checkedFirstRun = true
			m.currentScreen = ScreenClients
			initCmd := m.initScreen(ScreenClients)
			openFormCmd := func() tea.Msg { return OpenNewClientFormMsg{} }
			return m, tea.Batch(initCmd, openFormCmd)
		}
		m.checkedFirstRun = true
		return m, nil

	case SwitchScreenMsg:
		m.currentScreen = msg.Screen
		cmd := m.initScreen(msg.Screen)
		return m, cmd

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
	case ScreenSettings:
		if m.settings != nil {
			m.settings, cmd = m.settings.Update(msg)
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
	footer := footerStyle.Render("[T]imer  [E]ntries  [C]lients  [I]nvoices  [R]eports  [,] Settings  [Q]uit")

	// Current screen content
	var content string
	switch m.currentScreen {
	case ScreenDashboard:
		if m.dashboard != nil {
			content = m.dashboard.View()
		} else {
			content = "Loading..."
		}
	case ScreenTimer:
		if m.timer != nil {
			content = m.timer.View()
		} else {
			content = "Loading..."
		}
	case ScreenEntries:
		if m.entries != nil {
			content = m.entries.View()
		} else {
			content = "Loading..."
		}
	case ScreenClients:
		if m.clients != nil {
			content = m.clients.View()
		} else {
			content = "Loading..."
		}
	case ScreenInvoices:
		if m.invoices != nil {
			content = m.invoices.View()
		} else {
			content = "Loading..."
		}
	case ScreenReports:
		if m.reports != nil {
			content = m.reports.View()
		} else {
			content = "Loading..."
		}
	case ScreenSettings:
		if m.settings != nil {
			content = m.settings.View()
		} else {
			content = "Loading..."
		}
	}

	// Error/warning display
	errorDisplay := ""
	if m.quitMsg != "" {
		errorDisplay = lipgloss.NewStyle().
			Foreground(warningColor).
			Render(fmt.Sprintf("\n%s", m.quitMsg))
	} else if m.err != nil {
		errorDisplay = lipgloss.NewStyle().
			Foreground(errorColor).
			Render(fmt.Sprintf("\nError: %s", m.err.Error()))
	}

	// Divider line between header and content
	innerWidth := m.width - 6 // account for border (2) + padding (4)
	if innerWidth < 20 {
		innerWidth = 20
	}
	dividerWidth := innerWidth - 12
	if dividerWidth < 10 {
		dividerWidth = 10
	}
	divider := lipgloss.NewStyle().Foreground(borderColor).Render(
		strings.Repeat("─", dividerWidth),
	)

	body := fmt.Sprintf("%s\n%s\n\n%s%s\n\n%s\n%s", header, divider, content, errorDisplay, divider, footer)

	// Wrap in border, sized to terminal
	frame := appBorderStyle.
		Width(innerWidth).
		Height(m.height - 4) // leave room for border top/bottom
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, frame.Render(body))
}

// Run starts the TUI
func Run(a *app.App) error {
	p := tea.NewProgram(New(a), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
