package tui

import (
	"context"
	"fmt"
	"time"

	"github.com/andy/timesink/internal/app"
	"github.com/andy/timesink/internal/domain"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TimerTickMsg is sent every second when timer is running (screen-local)
type TimerTickMsg struct{}

// tickTimer returns a command that sends TimerTickMsg every second
func tickTimer() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return TimerTickMsg{}
	})
}

// clientsLoadedMsg is sent when clients are loaded
type clientsLoadedMsg struct {
	clients []*domain.Client
	err     error
}

// timerStoppedMsg is sent when a timer is stopped successfully
type timerStoppedMsg struct {
	entry *domain.TimeEntry
}

// loadClientsCmd loads the list of active clients
func loadClientsCmd(a *app.App) tea.Cmd {
	return func() tea.Msg {
		clients, err := a.ClientRepo.List(context.Background(), false)
		return clientsLoadedMsg{clients: clients, err: err}
	}
}

// TimerModel is a simple screen showing the active timer and controls
type TimerModel struct {
	app       *app.App
	timer     *domain.ActiveTimer
	clients   []*domain.Client
	client    *domain.Client // current timer's client
	err       error
	statusMsg string
}

// IsCapturingInput returns true when a timer is active so that keys like
// r (resume), s, p, d are not intercepted by global screen navigation.
func (m *TimerModel) IsCapturingInput() bool {
	return m.timer != nil
}

// NewTimerModel creates a new TimerModel
func NewTimerModel(a *app.App) tea.Model {
	m := &TimerModel{app: a}
	t, err := a.TimerService.GetActiveTimer(context.Background())
	if err != nil {
		m.err = err
	}
	m.timer = t
	return m
}

// Init starts the ticker when there's an active timer and loads clients
func (m *TimerModel) Init() tea.Cmd {
	var cmds []tea.Cmd
	cmds = append(cmds, loadClientsCmd(m.app))
	if m.timer != nil {
		cmds = append(cmds, tickTimer())
	}
	return tea.Batch(cmds...)
}

// Update handles key events and ticks
func (m *TimerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case RefreshDataMsg:
		var cmds []tea.Cmd
		cmds = append(cmds, loadClientsCmd(m.app))
		t, err := m.app.TimerService.GetActiveTimer(context.Background())
		if err != nil {
			m.err = err
		} else {
			m.timer = t
			if t != nil {
				m.loadTimerClient()
				cmds = append(cmds, tickTimer())
			}
		}
		return m, tea.Batch(cmds...)

	case clientsLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.clients = msg.clients
		if m.timer != nil {
			m.loadTimerClient()
		}
		return m, nil

	case timerStoppedMsg:
		m.timer = nil
		m.client = nil
		m.statusMsg = fmt.Sprintf("Entry saved: %.1fh",
			msg.entry.Duration().Hours())
		return m, nil

	case TimerTickMsg:
		// Only continue ticking if we have an active timer
		if m.timer == nil {
			return m, nil
		}
		t, err := m.app.TimerService.GetActiveTimer(context.Background())
		if err != nil {
			m.err = err
			return m, nil
		}
		if t == nil {
			// Timer was stopped externally (e.g. CLI)
			m.timer = nil
			m.client = nil
			return m, nil
		}
		m.timer = t
		return m, tickTimer()

	case tea.KeyMsg:
		m.err = nil
		m.statusMsg = ""

		switch msg.String() {
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			if m.timer == nil && m.clients != nil {
				idx := int(msg.String()[0] - '1')
				if idx >= 0 && idx < len(m.clients) && idx < 9 {
					return m, m.startTimer(m.clients[idx])
				}
			}
		case "s":
			if m.timer == nil && len(m.clients) > 0 {
				return m, m.startTimer(m.clients[0])
			}
		case "p":
			if m.timer != nil {
				if err := m.app.TimerService.Pause(context.Background()); err != nil {
					m.err = err
					return m, nil
				}
				m.timer, _ = m.app.TimerService.GetActiveTimer(context.Background())
			}
			return m, nil
		case "r":
			if m.timer != nil {
				if err := m.app.TimerService.Resume(context.Background()); err != nil {
					m.err = err
					return m, nil
				}
				m.timer, _ = m.app.TimerService.GetActiveTimer(context.Background())
				return m, tickTimer()
			}
		case "x":
			if m.timer != nil {
				return m, m.stopTimer()
			}
			return m, nil
		case "d":
			if m.timer != nil {
				if err := m.app.TimerService.Discard(context.Background()); err != nil {
					m.err = err
					return m, nil
				}
				m.timer = nil
				m.client = nil
				m.statusMsg = "Timer discarded"
			}
			return m, nil
		}
	}

	return m, nil
}

func (m *TimerModel) startTimer(client *domain.Client) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		if err := m.app.TimerService.Start(ctx, client.ID, ""); err != nil {
			return ErrorMsg{Err: err}
		}
		t, err := m.app.TimerService.GetActiveTimer(ctx)
		if err != nil {
			return ErrorMsg{Err: err}
		}
		m.timer = t
		m.client = client
		return TimerTickMsg{}
	}
}

func (m *TimerModel) stopTimer() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		entry, err := m.app.TimerService.Stop(ctx)
		if err != nil {
			return ErrorMsg{Err: err}
		}
		return timerStoppedMsg{entry: entry}
	}
}

// loadTimerClient loads the client details for the active timer
func (m *TimerModel) loadTimerClient() {
	if m.timer == nil {
		m.client = nil
		return
	}
	client, err := m.app.ClientRepo.GetByID(context.Background(), m.timer.ClientID)
	if err != nil {
		m.client = nil
		return
	}
	m.client = client
}

// View renders the timer screen
func (m *TimerModel) View() string {
	var b string
	title := lipgloss.NewStyle().Bold(true).Render("Active Timer")

	if m.err != nil {
		return title + "\n\n" +
			lipgloss.NewStyle().Foreground(errorColor).
				Render(fmt.Sprintf("Error: %s", m.err.Error())) +
			"\n\nPress any key to dismiss"
	}

	if m.timer == nil {
		// No active timer - show client selection
		b += title + "\n\n"

		if m.statusMsg != "" {
			b += lipgloss.NewStyle().Foreground(successColor).
				Render("  "+m.statusMsg) + "\n\n"
		}

		b += "No active timer. Select a client to start:\n\n"

		if m.clients == nil {
			b += "Loading clients...\n"
		} else if len(m.clients) == 0 {
			b += "No clients available. Add a client first.\n"
		} else {
			for i, client := range m.clients {
				if i >= 9 {
					break
				}
				shortcut := fmt.Sprintf("[%d]", i+1)
				rate := formatMoney(client.HourlyRate)
				b += fmt.Sprintf("%s %s (%s/hr)\n", shortcut, client.Name, rate)
			}
		}
		b += "\nKeys: 1-9=quick start, s=start with first client\n"
		return b
	}

	// Active timer view
	elapsed := m.timer.Elapsed()
	elapsedHours := elapsed.Hours()

	hours := int(elapsed.Hours())
	minutes := int(elapsed.Minutes()) % 60
	seconds := int(elapsed.Seconds()) % 60
	elapsedStr := fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)

	var clientName string
	var rate float64
	var valueAccrued float64

	if m.client != nil {
		clientName = m.client.Name
		rate = m.client.HourlyRate
		valueAccrued = elapsedHours * rate
	} else {
		clientName = fmt.Sprintf("Client #%d", m.timer.ClientID)
	}

	var stateStr string
	if m.timer.State() == domain.TimerStatePaused {
		stateStr = timerPausedStyle.Render("PAUSED")
	} else {
		stateStr = timerRunningStyle.Render("RUNNING")
	}

	b += title + "\n\n"
	b += fmt.Sprintf("State: %s\n", stateStr)
	b += fmt.Sprintf("Client: %s\n", clientName)
	if rate > 0 {
		b += fmt.Sprintf("Rate: %s/hr\n", formatMoney(rate))
	}
	if m.timer.Description != "" {
		b += fmt.Sprintf("Description: %s\n", m.timer.Description)
	}
	b += fmt.Sprintf("Started: %s\n", m.timer.StartTime.Format("2006-01-02 15:04:05"))
	b += fmt.Sprintf("Elapsed: %s\n", elapsedStr)
	if rate > 0 {
		valueStr := timerValueStyle.Render(formatMoney(valueAccrued))
		b += fmt.Sprintf("Value accrued: %s\n", valueStr)
	}
	b += "\nKeys: p=pause, r=resume, x=stop, d=discard\n"
	return b
}
