package tui

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/andy/timesink/internal/app"
	"github.com/andy/timesink/internal/service"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ReportsModel displays weekly, monthly, and financial reports
type ReportsModel struct {
	app       *app.App
	weekStart time.Time
	revenueYear int

	// Week data
	weekSummary *service.WeekSummary
	clientNames map[int64]string
	clientRates map[int64]float64

	// Daily detail
	dayCursor    int // 0=Mon, 6=Sun
	dailySummary *service.DailySummary

	// Financial data
	outstanding float64
	unbilled    float64
	monthly     map[time.Month]float64

	loading bool
	err     error
}

type reportsDataMsg struct {
	weekSummary *service.WeekSummary
	clientNames map[int64]string
	clientRates map[int64]float64
	outstanding float64
	unbilled    float64
	monthly     map[time.Month]float64
	err         error
}

type dailyDetailMsg struct {
	summary *service.DailySummary
	err     error
}

// NewReportsModel creates a new reports screen model
func NewReportsModel(a *app.App) tea.Model {
	return &ReportsModel{
		app:         a,
		weekStart:   weekMonday(time.Now()),
		revenueYear: time.Now().Year(),
		loading:     true,
	}
}

func (m *ReportsModel) Init() tea.Cmd {
	return m.loadData()
}

func (m *ReportsModel) loadData() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		msg := reportsDataMsg{
			clientNames: make(map[int64]string),
			clientRates: make(map[int64]float64),
		}

		// Week summary
		ws, err := m.app.ReportService.GetWeekSummary(ctx, m.weekStart)
		if err != nil {
			msg.err = err
			return msg
		}
		msg.weekSummary = ws

		// Resolve client names and rates
		for cid := range ws.ByClient {
			client, err := m.app.ClientRepo.GetByID(ctx, cid)
			if err == nil && client != nil {
				msg.clientNames[cid] = client.Name
				msg.clientRates[cid] = client.HourlyRate
			}
		}

		// Financial
		msg.outstanding, _ = m.app.ReportService.GetOutstandingTotal(ctx)
		msg.unbilled, _ = m.app.ReportService.GetUnbilledTotal(ctx)

		// Monthly revenue
		msg.monthly, _ = m.app.ReportService.GetRevenueByMonth(ctx, m.revenueYear)

		return msg
	}
}

func (m *ReportsModel) loadDailyDetail() tea.Cmd {
	selectedDate := m.weekStart.AddDate(0, 0, m.dayCursor)
	return func() tea.Msg {
		ctx := context.Background()
		summary, err := m.app.ReportService.GetDailySummary(ctx, selectedDate)
		if err != nil {
			return dailyDetailMsg{err: err}
		}
		return dailyDetailMsg{summary: summary}
	}
}

func (m *ReportsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case RefreshDataMsg:
		m.loading = true
		return m, m.loadData()

	case reportsDataMsg:
		m.loading = false
		m.err = msg.err
		if msg.err == nil {
			m.weekSummary = msg.weekSummary
			m.clientNames = msg.clientNames
			m.clientRates = msg.clientRates
			m.outstanding = msg.outstanding
			m.unbilled = msg.unbilled
			m.monthly = msg.monthly
		}
		// Load daily detail for current cursor
		if msg.err == nil {
			return m, m.loadDailyDetail()
		}
		return m, nil

	case dailyDetailMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.dailySummary = msg.summary
		return m, nil

	case tea.KeyMsg:
		if m.loading {
			return m, nil
		}

		switch {
		case key.Matches(msg, DefaultKeyMap.Left):
			// Previous week
			m.weekStart = m.weekStart.AddDate(0, 0, -7)
			m.dailySummary = nil
			m.loading = true
			return m, m.loadData()

		case key.Matches(msg, DefaultKeyMap.Right):
			// Next week
			next := m.weekStart.AddDate(0, 0, 7)
			if !next.After(time.Now()) {
				m.weekStart = next
				m.dailySummary = nil
				m.loading = true
				return m, m.loadData()
			}

		case key.Matches(msg, DefaultKeyMap.Up):
			if m.dayCursor > 0 {
				m.dayCursor--
				return m, m.loadDailyDetail()
			}

		case key.Matches(msg, DefaultKeyMap.Down):
			if m.dayCursor < 6 {
				m.dayCursor++
				return m, m.loadDailyDetail()
			}

		case msg.String() == "[":
			// Previous year for revenue
			m.revenueYear--
			m.loading = true
			return m, m.loadData()

		case msg.String() == "]":
			// Next year for revenue
			if m.revenueYear < time.Now().Year() {
				m.revenueYear++
				m.loading = true
				return m, m.loadData()
			}
		}
	}

	return m, nil
}

func (m *ReportsModel) View() string {
	if m.loading {
		return titleStyle.Render("Reports") + "\n\n  Loading..."
	}

	if m.err != nil {
		return titleStyle.Render("Reports") + "\n\n" +
			lipgloss.NewStyle().Foreground(errorColor).Render(fmt.Sprintf("  Error: %v", m.err))
	}

	var s string

	// Title and week navigation
	weekEnd := m.weekStart.AddDate(0, 0, 6)
	s += titleStyle.Render("Reports") + "\n"
	s += fmt.Sprintf("  Week of %s - %s\n\n",
		m.weekStart.Format("Jan 2"),
		weekEnd.Format("Jan 2, 2006"),
	)

	// Weekly hours bar chart with day selection
	s += lipgloss.NewStyle().Bold(true).Render("  Hours by Day") + "\n"
	s += m.renderWeekChart()
	s += "\n"

	// Weekly totals with utilization
	s += m.renderWeekTotals()
	s += "\n"

	// Daily detail for selected day
	s += m.renderDailyDetail()
	s += "\n"

	// Hours & value by client
	s += m.renderClientBreakdown()

	// Financial overview
	s += lipgloss.NewStyle().Bold(true).Render("  Financial Overview") + "\n"
	s += fmt.Sprintf("    Outstanding: %s\n", formatMoney(m.outstanding))
	s += fmt.Sprintf("    Unbilled:    %s\n", formatMoney(m.unbilled))
	s += "\n"

	// Monthly revenue
	s += m.renderMonthlyRevenue()

	// Key help
	s += "\n" + helpStyle.Render("  j/k: select day  h/l: prev/next week  [/]: prev/next year")

	return s
}

func (m *ReportsModel) renderWeekChart() string {
	days := []time.Weekday{
		time.Monday, time.Tuesday, time.Wednesday,
		time.Thursday, time.Friday, time.Saturday, time.Sunday,
	}

	ws := m.weekSummary
	if ws == nil {
		return "    No data\n"
	}

	// Find max for scaling
	maxHours := 0.0
	for _, h := range ws.ByDay {
		if h > maxHours {
			maxHours = h
		}
	}

	maxBar := 25
	var chart string
	for i, day := range days {
		hours := ws.ByDay[day]
		barLen := 0
		if maxHours > 0 {
			barLen = int((hours / maxHours) * float64(maxBar))
		}
		bar := ""
		for j := 0; j < barLen; j++ {
			bar += "█"
		}

		selected := i == m.dayCursor

		dayName := day.String()[:3]
		dateStr := m.weekStart.AddDate(0, 0, i).Format("Jan 2")
		label := fmt.Sprintf("%s %s", dayName, dateStr)

		dayStyle := lipgloss.NewStyle().Width(12)
		barStyle := lipgloss.NewStyle().Foreground(primaryColor)
		hoursStr := formatHours(hours)

		line := fmt.Sprintf("    %s %s %s",
			dayStyle.Render(label),
			barStyle.Render(fmt.Sprintf("%-25s", bar)),
			hoursStr,
		)

		if selected {
			chart += lipgloss.NewStyle().Bold(true).Foreground(primaryColor).Render(fmt.Sprintf("  > %s %s %s",
				dayStyle.Render(label),
				barStyle.Render(fmt.Sprintf("%-25s", bar)),
				hoursStr,
			)) + "\n"
		} else {
			chart += line + "\n"
		}
	}

	return chart
}

func (m *ReportsModel) renderWeekTotals() string {
	ws := m.weekSummary
	if ws == nil {
		return ""
	}

	s := lipgloss.NewStyle().Bold(true).Render("  Weekly Totals") + "\n"
	s += fmt.Sprintf("    Total:       %s\n", formatHours(ws.TotalHours))
	s += fmt.Sprintf("    Billable:    %s\n", formatHours(ws.BillableHours))
	s += fmt.Sprintf("    Value:       %s\n", formatMoney(ws.TotalValue))

	// Utilization rate
	if ws.TotalHours > 0 {
		utilization := (ws.BillableHours / ws.TotalHours) * 100
		utilStr := fmt.Sprintf("%.0f%%", utilization)
		style := lipgloss.NewStyle()
		if utilization >= 80 {
			style = style.Foreground(successColor)
		} else if utilization >= 50 {
			style = style.Foreground(warningColor)
		} else {
			style = style.Foreground(errorColor)
		}
		s += fmt.Sprintf("    Utilization: %s\n", style.Render(utilStr))
	}

	return s
}

func (m *ReportsModel) renderDailyDetail() string {
	days := []time.Weekday{
		time.Monday, time.Tuesday, time.Wednesday,
		time.Thursday, time.Friday, time.Saturday, time.Sunday,
	}

	selectedDate := m.weekStart.AddDate(0, 0, m.dayCursor)
	dayName := days[m.dayCursor].String()
	header := fmt.Sprintf("  %s, %s", dayName, selectedDate.Format("January 2"))
	s := lipgloss.NewStyle().Bold(true).Render(header) + "\n"

	if m.dailySummary == nil || len(m.dailySummary.Entries) == 0 {
		s += subtitleStyle.Render("    No entries") + "\n"
		return s
	}

	ds := m.dailySummary
	s += subtitleStyle.Render(fmt.Sprintf("    %s total  |  %s billable  |  %s",
		formatHours(ds.TotalHours),
		formatHours(ds.BillableHours),
		formatMoney(ds.TotalValue),
	)) + "\n"

	for _, entry := range ds.Entries {
		clientName := m.clientNames[entry.ClientID]
		if clientName == "" {
			clientName = fmt.Sprintf("Client #%d", entry.ClientID)
		}

		timeRange := fmt.Sprintf("%s-%s",
			entry.StartTime.Format("15:04"),
			entry.EndTime.Format("15:04"),
		)

		desc := entry.Description
		if desc == "" {
			desc = "(no description)"
		}
		desc = truncateStr(desc, 30)

		billable := " "
		if !entry.IsBillable {
			billable = lipgloss.NewStyle().Foreground(mutedColor).Render("nb")
		}

		line := fmt.Sprintf("    %s  %-15s  %s  %10s  %s",
			timeRange,
			truncateStr(clientName, 15),
			formatHours(entry.Duration().Hours()),
			formatMoney(entry.Amount()),
			billable,
		)

		if !entry.IsBillable {
			s += lipgloss.NewStyle().Foreground(mutedColor).Render(line) + "\n"
		} else {
			s += line + "\n"
		}

		// Description on second line
		s += subtitleStyle.Render(fmt.Sprintf("              %s", desc)) + "\n"
	}

	return s
}

func (m *ReportsModel) renderClientBreakdown() string {
	ws := m.weekSummary
	if ws == nil || len(ws.ByClient) == 0 {
		return ""
	}

	s := lipgloss.NewStyle().Bold(true).Render("  Hours & Value by Client") + "\n"

	// Sort clients by hours descending
	type clientEntry struct {
		id    int64
		hours float64
	}
	var sorted []clientEntry
	for cid, hours := range ws.ByClient {
		sorted = append(sorted, clientEntry{id: cid, hours: hours})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].hours > sorted[j].hours
	})

	for _, ce := range sorted {
		name := m.clientNames[ce.id]
		if name == "" {
			name = fmt.Sprintf("Client #%d", ce.id)
		}
		rate := m.clientRates[ce.id]
		value := ce.hours * rate

		s += fmt.Sprintf("    %-20s  %s  %s",
			truncateStr(name, 20),
			formatHours(ce.hours),
			formatMoney(value),
		)
		if rate > 0 {
			s += subtitleStyle.Render(fmt.Sprintf("  @ %s/hr", formatMoney(rate)))
		}
		s += "\n"
	}

	s += "\n"
	return s
}

func (m *ReportsModel) renderMonthlyRevenue() string {
	s := lipgloss.NewStyle().Bold(true).Render(
		fmt.Sprintf("  Revenue by Month (%d)", m.revenueYear),
	) + "\n"

	months := []time.Month{
		time.January, time.February, time.March,
		time.April, time.May, time.June,
		time.July, time.August, time.September,
		time.October, time.November, time.December,
	}

	hasRevenue := false
	yearTotal := 0.0
	for _, month := range months {
		revenue := m.monthly[month]
		if revenue > 0 {
			hasRevenue = true
			yearTotal += revenue
			s += fmt.Sprintf("    %-10s %s\n", month.String()[:3], formatMoney(revenue))
		}
	}

	if !hasRevenue {
		s += subtitleStyle.Render("    No revenue recorded") + "\n"
	} else {
		s += "    " + lipgloss.NewStyle().Bold(true).Render(
			fmt.Sprintf("%-10s %s", "Total", formatMoney(yearTotal)),
		) + "\n"
	}

	return s
}

// weekMonday returns the Monday of the week containing t
func weekMonday(t time.Time) time.Time {
	for t.Weekday() != time.Monday {
		t = t.AddDate(0, 0, -1)
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}
