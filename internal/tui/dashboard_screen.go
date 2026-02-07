package tui

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/andy/timesink/internal/app"
	"github.com/andy/timesink/internal/domain"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// DashboardModel represents the dashboard home screen
type DashboardModel struct {
	app *app.App

	// Data
	weekTotalHours    float64
	weekBillableHours float64
	weekTotalValue    float64
	todayTotalHours   float64
	todayTotalValue   float64
	outstanding       float64
	unbilled          float64
	activeTimer       *domain.ActiveTimer
	activeClient      *domain.Client
	recentEntries     []*domain.TimeEntry
	clientCache       map[int64]*domain.Client

	loading bool
	err     error
}

type dashboardDataMsg struct {
	weekTotalHours    float64
	weekBillableHours float64
	weekTotalValue    float64
	todayTotalHours   float64
	todayTotalValue   float64
	outstanding       float64
	unbilled          float64
	activeTimer       *domain.ActiveTimer
	activeClient      *domain.Client
	recentEntries     []*domain.TimeEntry
	clientCache       map[int64]*domain.Client
	err               error
}

// NewDashboardModel creates a new dashboard model
func NewDashboardModel(a *app.App) tea.Model {
	return &DashboardModel{
		app:         a,
		loading:     true,
		clientCache: make(map[int64]*domain.Client),
	}
}

func (m *DashboardModel) Init() tea.Cmd {
	return m.loadData()
}

func (m *DashboardModel) loadData() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		msg := dashboardDataMsg{
			clientCache: make(map[int64]*domain.Client),
		}

		now := time.Now()

		// Week start (Monday)
		weekStart := now
		for weekStart.Weekday() != time.Monday {
			weekStart = weekStart.AddDate(0, 0, -1)
		}
		weekStart = time.Date(weekStart.Year(), weekStart.Month(), weekStart.Day(), 0, 0, 0, 0, weekStart.Location())

		// Load week summary
		weekSummary, err := m.app.ReportService.GetWeekSummary(ctx, weekStart)
		if err != nil {
			msg.err = fmt.Errorf("week summary: %w", err)
			return msg
		}
		msg.weekTotalHours = weekSummary.TotalHours
		msg.weekBillableHours = weekSummary.BillableHours
		msg.weekTotalValue = weekSummary.TotalValue

		// Load today summary
		dailySummary, err := m.app.ReportService.GetDailySummary(ctx, now)
		if err != nil {
			msg.err = fmt.Errorf("daily summary: %w", err)
			return msg
		}
		msg.todayTotalHours = dailySummary.TotalHours
		msg.todayTotalValue = dailySummary.TotalValue

		// Financial totals
		msg.outstanding, _ = m.app.ReportService.GetOutstandingTotal(ctx)
		msg.unbilled, _ = m.app.ReportService.GetUnbilledTotal(ctx)

		// Active timer
		activeTimer, err := m.app.TimerService.GetActiveTimer(ctx)
		if err == nil && activeTimer != nil {
			msg.activeTimer = activeTimer
			client, err := m.app.ClientRepo.GetByID(ctx, activeTimer.ClientID)
			if err == nil {
				msg.activeClient = client
				msg.clientCache[client.ID] = client
			}
		}

		// Recent entries (last 7 days)
		sevenDaysAgo := now.AddDate(0, 0, -7)
		entries, err := m.app.EntryRepo.List(ctx, nil, &sevenDaysAgo, &now, true)
		if err == nil {
			msg.recentEntries = entries
			for _, entry := range entries {
				if _, ok := msg.clientCache[entry.ClientID]; !ok {
					client, err := m.app.ClientRepo.GetByID(ctx, entry.ClientID)
					if err == nil {
						msg.clientCache[entry.ClientID] = client
					}
				}
			}
		}

		return msg
	}
}

func (m *DashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case dashboardDataMsg:
		m.loading = false
		m.err = msg.err
		m.weekTotalHours = msg.weekTotalHours
		m.weekBillableHours = msg.weekBillableHours
		m.weekTotalValue = msg.weekTotalValue
		m.todayTotalHours = msg.todayTotalHours
		m.todayTotalValue = msg.todayTotalValue
		m.outstanding = msg.outstanding
		m.unbilled = msg.unbilled
		m.activeTimer = msg.activeTimer
		m.activeClient = msg.activeClient
		m.recentEntries = msg.recentEntries
		m.clientCache = msg.clientCache
		if m.activeTimer != nil {
			return m, tickTimer()
		}
		return m, nil

	case TimerTickMsg:
		if m.activeTimer != nil {
			// Refresh timer state
			ctx := context.Background()
			t, err := m.app.TimerService.GetActiveTimer(ctx)
			if err == nil {
				m.activeTimer = t
			}
			return m, tickTimer()
		}
		return m, nil

	case RefreshDataMsg:
		m.loading = true
		return m, m.loadData()
	}

	return m, nil
}

func (m *DashboardModel) View() string {
	if m.loading {
		return "Loading dashboard..."
	}

	if m.err != nil {
		return lipgloss.NewStyle().Foreground(errorColor).
			Render(fmt.Sprintf("Error: %v", m.err))
	}

	var s string

	// Summary boxes
	summaryLeft := fmt.Sprintf(
		"  This Week:  %-12s  Billable:     %s\n  Today:      %-12s  Outstanding:  %s",
		formatHours(m.weekTotalHours),
		formatMoney(m.weekTotalValue),
		formatHours(m.todayTotalHours),
		formatMoney(m.outstanding),
	)
	s += summaryLeft + "\n"

	// Active timer
	s += "\n"
	if m.activeTimer != nil {
		s += m.renderActiveTimer()
	} else {
		s += subtitleStyle.Render("  No active timer") + "\n"
	}

	// Recent entries
	s += "\n" + m.renderRecentEntries()

	return s
}

func (m *DashboardModel) renderActiveTimer() string {
	clientName := fmt.Sprintf("Client #%d", m.activeTimer.ClientID)
	if m.activeClient != nil {
		clientName = m.activeClient.Name
	}

	elapsed := m.activeTimer.Elapsed()
	h := int(elapsed.Hours())
	min := int(elapsed.Minutes()) % 60
	sec := int(elapsed.Seconds()) % 60
	timeStr := fmt.Sprintf("%02d:%02d:%02d", h, min, sec)

	var stateStyle lipgloss.Style
	if m.activeTimer.State() == domain.TimerStatePaused {
		stateStyle = timerPausedStyle
	} else {
		stateStyle = timerRunningStyle
	}

	return fmt.Sprintf("  Active Timer\n  %s %s - %s  [%s]\n",
		stateStyle.Render("●"),
		clientName,
		m.activeTimer.Description,
		timerValueStyle.Render(timeStr),
	)
}

func (m *DashboardModel) renderRecentEntries() string {
	header := "  Recent Entries (Last 7 Days)\n"
	if len(m.recentEntries) == 0 {
		return header + subtitleStyle.Render("  No recent entries") + "\n"
	}

	// Sort most recent first
	sorted := make([]*domain.TimeEntry, len(m.recentEntries))
	copy(sorted, m.recentEntries)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].StartTime.After(sorted[j].StartTime)
	})

	s := header
	limit := 8
	if len(sorted) < limit {
		limit = len(sorted)
	}

	for i := 0; i < limit; i++ {
		entry := sorted[i]
		clientName := fmt.Sprintf("Client #%d", entry.ClientID)
		if c, ok := m.clientCache[entry.ClientID]; ok {
			clientName = c.Name
		}

		hours := entry.Duration().Hours()
		desc := truncateStr(entry.Description, 30)

		s += fmt.Sprintf("  %-7s %-20s %6s  %s\n",
			entry.StartTime.Format("Jan 2"),
			truncateStr(clientName, 20),
			formatHours(hours),
			desc,
		)
	}

	return s
}
