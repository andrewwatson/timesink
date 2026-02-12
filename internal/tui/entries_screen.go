package tui

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/andy/timesink/internal/app"
	"github.com/andy/timesink/internal/domain"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type entryMode int

const (
	entryModeList          entryMode = iota
	entryModePickClient              // cursor-based client selection
	entryModeNew                     // text input form for entry details
	entryModeConfirmDelete           // y/n confirmation before delete
	entryModeEditDesc                // inline description editing
)

// entry form field indices (after client is selected)
const (
	entryFieldDate = iota
	entryFieldStartTime
	entryFieldEndTime
	entryFieldDescription
	entryFieldRate
	entryFieldCount
)

// EntriesModel displays a scrollable list of time entries
type EntriesModel struct {
	app         *app.App
	entries     []*domain.TimeEntry
	clientNames map[int64]string
	cursor      int
	offset      int
	maxVisible  int
	loading     bool
	err         error
	statusMsg   string

	// Form state
	mode        entryMode
	fields      []textinput.Model
	fieldFocus  int
	formClients []*domain.Client
	formClient  *domain.Client // selected client
	clientCursor int

	// Inline description editing
	descInput textinput.Model
}

type entriesDataMsg struct {
	entries     []*domain.TimeEntry
	clientNames map[int64]string
	err         error
}

type entrySavedMsg struct {
	err error
}

type entryClientsMsg struct {
	clients []*domain.Client
	err     error
}

type entryDeletedMsg struct {
	err error
}

type entryDescUpdatedMsg struct {
	err error
}

// IsCapturingInput returns true when the text form or delete confirmation is active
func (m *EntriesModel) IsCapturingInput() bool {
	return m.mode == entryModeNew || m.mode == entryModeConfirmDelete || m.mode == entryModeEditDesc
}

// NewEntriesModel creates a new entries screen model
func NewEntriesModel(a *app.App) tea.Model {
	return &EntriesModel{
		app:         a,
		clientNames: make(map[int64]string),
		maxVisible:  15,
		loading:     true,
	}
}

func (m *EntriesModel) Init() tea.Cmd {
	return m.loadEntries()
}

func (m *EntriesModel) loadEntries() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		end := time.Now()
		start := end.AddDate(0, 0, -30)

		entries, err := m.app.EntryRepo.List(ctx, nil, &start, &end, true)
		if err != nil {
			return entriesDataMsg{err: err}
		}

		// Resolve client names
		clientNames := make(map[int64]string)
		for _, entry := range entries {
			if _, ok := clientNames[entry.ClientID]; !ok {
				client, err := m.app.ClientRepo.GetByID(ctx, entry.ClientID)
				if err == nil && client != nil {
					clientNames[entry.ClientID] = client.Name
				} else {
					clientNames[entry.ClientID] = fmt.Sprintf("Client #%d", entry.ClientID)
				}
			}
		}

		return entriesDataMsg{
			entries:     entries,
			clientNames: clientNames,
		}
	}
}

func (m *EntriesModel) loadFormClients() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		clients, err := m.app.ClientRepo.List(ctx, false)
		if err != nil {
			return entryClientsMsg{err: err}
		}
		return entryClientsMsg{clients: clients}
	}
}

func (m *EntriesModel) selectClient(client *domain.Client) {
	m.formClient = client
	m.initForm()
	m.mode = entryModeNew
}

func (m *EntriesModel) initForm() {
	m.fields = make([]textinput.Model, entryFieldCount)

	// Date
	m.fields[entryFieldDate] = textinput.New()
	m.fields[entryFieldDate].Placeholder = "2006-01-02"
	m.fields[entryFieldDate].CharLimit = 10
	m.fields[entryFieldDate].Width = 15
	m.fields[entryFieldDate].SetValue(time.Now().Format("2006-01-02"))

	// Start time
	m.fields[entryFieldStartTime] = textinput.New()
	m.fields[entryFieldStartTime].Placeholder = "09:00"
	m.fields[entryFieldStartTime].CharLimit = 5
	m.fields[entryFieldStartTime].Width = 10

	// End time
	m.fields[entryFieldEndTime] = textinput.New()
	m.fields[entryFieldEndTime].Placeholder = "17:00"
	m.fields[entryFieldEndTime].CharLimit = 5
	m.fields[entryFieldEndTime].Width = 10

	// Description
	m.fields[entryFieldDescription] = textinput.New()
	m.fields[entryFieldDescription].Placeholder = "What did you work on?"
	m.fields[entryFieldDescription].CharLimit = 200
	m.fields[entryFieldDescription].Width = 50

	// Hourly rate — pre-fill from selected client
	m.fields[entryFieldRate] = textinput.New()
	m.fields[entryFieldRate].Placeholder = "150.00"
	m.fields[entryFieldRate].CharLimit = 10
	m.fields[entryFieldRate].Width = 15
	if m.formClient != nil {
		m.fields[entryFieldRate].SetValue(fmt.Sprintf("%.2f", m.formClient.HourlyRate))
	}

	m.fieldFocus = entryFieldDate
	m.fields[entryFieldDate].Focus()
}

func (m *EntriesModel) saveEntry() tea.Cmd {
	client := m.formClient
	dateStr := m.fields[entryFieldDate].Value()
	startStr := m.fields[entryFieldStartTime].Value()
	endStr := m.fields[entryFieldEndTime].Value()
	desc := m.fields[entryFieldDescription].Value()
	rateStr := m.fields[entryFieldRate].Value()

	return func() tea.Msg {
		ctx := context.Background()

		// Parse date
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			return entrySavedMsg{err: fmt.Errorf("invalid date (use YYYY-MM-DD): %s", dateStr)}
		}

		// Parse start time
		startParts, err := time.Parse("15:04", startStr)
		if err != nil {
			return entrySavedMsg{err: fmt.Errorf("invalid start time (use HH:MM): %s", startStr)}
		}
		startTime := time.Date(date.Year(), date.Month(), date.Day(),
			startParts.Hour(), startParts.Minute(), 0, 0, time.Local)

		// Parse end time
		endParts, err := time.Parse("15:04", endStr)
		if err != nil {
			return entrySavedMsg{err: fmt.Errorf("invalid end time (use HH:MM): %s", endStr)}
		}
		endTime := time.Date(date.Year(), date.Month(), date.Day(),
			endParts.Hour(), endParts.Minute(), 0, 0, time.Local)

		if !endTime.After(startTime) {
			return entrySavedMsg{err: fmt.Errorf("end time must be after start time")}
		}

		// Parse rate
		rate, err := strconv.ParseFloat(rateStr, 64)
		if err != nil || rate < 0 {
			return entrySavedMsg{err: fmt.Errorf("invalid hourly rate: %s", rateStr)}
		}

		// Create entry
		entry := &domain.TimeEntry{
			ClientID:    client.ID,
			Description: desc,
			StartTime:   startTime,
			HourlyRate:  rate,
			IsBillable:  true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		entry.Stop(endTime)

		if err := m.app.EntryRepo.Create(ctx, entry); err != nil {
			return entrySavedMsg{err: err}
		}

		return entrySavedMsg{}
	}
}

func (m *EntriesModel) deleteEntry(id int64) tea.Cmd {
	return func() tea.Msg {
		err := m.app.EntryRepo.SoftDelete(context.Background(), id, "deleted by user")
		return entryDeletedMsg{err: err}
	}
}

func (m *EntriesModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle client loading result — arrives while still in list mode
	if msg, ok := msg.(entryClientsMsg); ok {
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		if len(msg.clients) == 0 {
			m.err = fmt.Errorf("no clients found — add a client first")
			return m, nil
		}
		m.formClients = msg.clients
		m.clientCursor = 0
		// Skip picker if only one client
		if len(msg.clients) == 1 {
			m.selectClient(msg.clients[0])
			return m, m.fields[m.fieldFocus].Focus()
		}
		m.mode = entryModePickClient
		return m, nil
	}

	// Route messages based on mode
	switch m.mode {
	case entryModePickClient:
		return m.updatePickClient(msg)
	case entryModeNew:
		return m.updateForm(msg)
	case entryModeConfirmDelete:
		return m.updateConfirmDelete(msg)
	case entryModeEditDesc:
		return m.updateEditDesc(msg)
	}

	switch msg := msg.(type) {
	case RefreshDataMsg:
		m.loading = true
		return m, m.loadEntries()

	case entriesDataMsg:
		m.loading = false
		m.err = msg.err
		if msg.err == nil {
			m.entries = msg.entries
			m.clientNames = msg.clientNames
		}
		return m, nil

	case tea.KeyMsg:
		if m.loading {
			return m, nil
		}

		m.statusMsg = ""
		m.err = nil

		switch {
		case key.Matches(msg, DefaultKeyMap.Up):
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.offset {
					m.offset = m.cursor
				}
			}
		case key.Matches(msg, DefaultKeyMap.Down):
			if m.cursor < len(m.entries)-1 {
				m.cursor++
				if m.cursor >= m.offset+m.maxVisible {
					m.offset = m.cursor - m.maxVisible + 1
				}
			}
		case msg.String() == "n":
			m.loading = true
			return m, m.loadFormClients()
		case msg.String() == "enter":
			if len(m.entries) > 0 && m.cursor < len(m.entries) {
				entry := m.entries[m.cursor]
				if entry.IsLocked() {
					m.err = fmt.Errorf("cannot edit: entry is locked by an invoice")
					return m, nil
				}
				ti := textinput.New()
				ti.Placeholder = "Enter description..."
				ti.SetValue(entry.Description)
				ti.CharLimit = 200
				ti.Width = 50
				m.descInput = ti
				m.mode = entryModeEditDesc
				return m, m.descInput.Focus()
			}
		case msg.String() == "d":
			if len(m.entries) > 0 && m.cursor < len(m.entries) {
				entry := m.entries[m.cursor]
				if entry.IsLocked() {
					m.err = fmt.Errorf("cannot delete: entry is locked by an invoice")
					return m, nil
				}
				m.mode = entryModeConfirmDelete
				return m, nil
			}
		}
	}

	return m, nil
}

func (m *EntriesModel) updatePickClient(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, DefaultKeyMap.Back):
			m.mode = entryModeList
			m.formClients = nil
			return m, nil
		case key.Matches(msg, DefaultKeyMap.Up):
			if m.clientCursor > 0 {
				m.clientCursor--
			}
		case key.Matches(msg, DefaultKeyMap.Down):
			if m.clientCursor < len(m.formClients)-1 {
				m.clientCursor++
			}
		case key.Matches(msg, DefaultKeyMap.Select):
			if len(m.formClients) > 0 {
				m.selectClient(m.formClients[m.clientCursor])
				return m, m.fields[m.fieldFocus].Focus()
			}
		}
	}
	return m, nil
}

func (m *EntriesModel) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case entrySavedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.mode = entryModeList
		m.statusMsg = "Entry saved"
		m.loading = true
		return m, m.loadEntries()

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.mode = entryModePickClient
			m.err = nil
			// Go back to client picker (or list if only one client)
			if len(m.formClients) <= 1 {
				m.mode = entryModeList
			}
			return m, nil

		case "tab", "down":
			m.fields[m.fieldFocus].Blur()
			m.fieldFocus = (m.fieldFocus + 1) % entryFieldCount
			return m, m.fields[m.fieldFocus].Focus()

		case "shift+tab", "up":
			m.fields[m.fieldFocus].Blur()
			m.fieldFocus = (m.fieldFocus - 1 + entryFieldCount) % entryFieldCount
			return m, m.fields[m.fieldFocus].Focus()

		case "enter":
			if m.fieldFocus == entryFieldCount-1 {
				return m, m.saveEntry()
			}
			m.fields[m.fieldFocus].Blur()
			m.fieldFocus++
			return m, m.fields[m.fieldFocus].Focus()

		case "ctrl+s":
			return m, m.saveEntry()
		}
	}

	// Update the focused text input
	var cmd tea.Cmd
	m.fields[m.fieldFocus], cmd = m.fields[m.fieldFocus].Update(msg)
	return m, cmd
}

func (m *EntriesModel) updateEditDesc(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case entryDescUpdatedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.mode = entryModeList
			return m, nil
		}
		m.mode = entryModeList
		m.statusMsg = "Description updated"
		m.loading = true
		return m, m.loadEntries()

	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			entry := m.entries[m.cursor]
			desc := m.descInput.Value()
			return m, func() tea.Msg {
				entry.Description = desc
				entry.UpdatedAt = time.Now()
				err := m.app.EntryRepo.Update(context.Background(), entry, "description updated")
				return entryDescUpdatedMsg{err: err}
			}
		case "esc":
			m.mode = entryModeList
			return m, nil
		default:
			var cmd tea.Cmd
			m.descInput, cmd = m.descInput.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m *EntriesModel) updateConfirmDelete(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case entryDeletedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.mode = entryModeList
			return m, nil
		}
		m.mode = entryModeList
		m.statusMsg = "Entry deleted"
		m.loading = true
		return m, m.loadEntries()

	case tea.KeyMsg:
		switch msg.String() {
		case "y":
			entry := m.entries[m.cursor]
			return m, m.deleteEntry(entry.ID)
		default:
			// Any other key cancels
			m.mode = entryModeList
			return m, nil
		}
	}
	return m, nil
}

func (m *EntriesModel) View() string {
	if m.loading {
		return "Loading entries..."
	}

	switch m.mode {
	case entryModePickClient:
		return m.viewPickClient()
	case entryModeNew:
		return m.viewForm()
	case entryModeConfirmDelete:
		return m.viewConfirmDelete()
	case entryModeEditDesc:
		return m.viewEditDesc()
	default:
		return m.viewList()
	}
}

func (m *EntriesModel) viewEditDesc() string {
	entry := m.entries[m.cursor]
	clientName := m.clientNames[entry.ClientID]
	date := entry.StartTime.Format("Jan 2")
	hours := formatHours(entry.Duration().Hours())

	var s string
	s += titleStyle.Render("Edit Description") + "\n\n"
	s += fmt.Sprintf("  %s  %s  %s\n\n", date, clientName, hours)
	s += fmt.Sprintf("  Description: %s\n\n", m.descInput.View())
	s += helpStyle.Render("  enter: save  esc: cancel") + "\n"
	return s
}

func (m *EntriesModel) viewConfirmDelete() string {
	entry := m.entries[m.cursor]
	clientName := m.clientNames[entry.ClientID]
	date := entry.StartTime.Format("Jan 2")
	hours := formatHours(entry.Duration().Hours())
	desc := truncateStr(entry.Description, 40)

	var s string
	s += titleStyle.Render("Delete Entry") + "\n\n"
	s += fmt.Sprintf("  %s  %s  %s  %s\n\n", date, clientName, hours, desc)
	s += lipgloss.NewStyle().Foreground(warningColor).Render("  Delete this entry? (y/n)") + "\n"
	return s
}

func (m *EntriesModel) viewList() string {
	if m.err != nil {
		return lipgloss.NewStyle().Foreground(errorColor).
			Render(fmt.Sprintf("Error: %v", m.err))
	}

	var s string

	s += titleStyle.Render("Time Entries") + "\n"

	if m.statusMsg != "" {
		s += lipgloss.NewStyle().Foreground(successColor).
			Render("  "+m.statusMsg) + "\n"
	}

	if len(m.entries) == 0 {
		s += "\n" + subtitleStyle.Render("  No time entries yet. Press 'n' to add one.")
		return s
	}

	// Summary
	totalHours, totalValue := m.calcTotals()
	s += subtitleStyle.Render(fmt.Sprintf(
		"  %d entries  |  %s total  |  %s value",
		len(m.entries), formatHours(totalHours), formatMoney(totalValue),
	)) + "\n\n"

	// Column header
	s += subtitleStyle.Render(fmt.Sprintf(
		"     %-7s  %-20s  %6s  %10s  %s",
		"Date", "Client", "Hours", "Amount", "Description",
	)) + "\n"

	// Entries
	end := m.offset + m.maxVisible
	if end > len(m.entries) {
		end = len(m.entries)
	}

	for i := m.offset; i < end; i++ {
		entry := m.entries[i]
		s += m.renderEntry(entry, i == m.cursor) + "\n"
	}

	// Scroll indicators
	if m.offset > 0 {
		s += subtitleStyle.Render("  ... more above") + "\n"
	}
	if end < len(m.entries) {
		s += subtitleStyle.Render("  ... more below") + "\n"
	}

	// Totals
	s += "\n" + lipgloss.NewStyle().Bold(true).Render(
		fmt.Sprintf("     %-7s  %-20s  %6s  %10s", "Total", "", formatHours(totalHours), formatMoney(totalValue)),
	) + "\n"

	s += "\n" + helpStyle.Render("  j/k: navigate  n: new entry  enter: edit desc  d: delete")

	return s
}

func (m *EntriesModel) viewPickClient() string {
	var s string
	s += titleStyle.Render("New Entry - Select Client") + "\n\n"

	for i, client := range m.formClients {
		indicator := "  "
		if i == m.clientCursor {
			indicator = "> "
		}

		rate := fmt.Sprintf("$%.0f/hr", client.HourlyRate)
		clientLine := fmt.Sprintf("%s%-25s  %s", indicator, client.Name, rate)

		if i == m.clientCursor {
			s += lipgloss.NewStyle().Bold(true).Foreground(primaryColor).Render(clientLine) + "\n"
		} else {
			s += clientLine + "\n"
		}
	}

	s += "\n" + helpStyle.Render("  j/k: navigate  enter: select  esc: cancel")

	return s
}

func (m *EntriesModel) viewForm() string {
	var s string

	clientName := ""
	if m.formClient != nil {
		clientName = m.formClient.Name
	}
	s += titleStyle.Render(fmt.Sprintf("New Entry - %s", clientName)) + "\n\n"

	labels := []string{"Date:", "Start Time:", "End Time:", "Description:", "Rate ($/hr):"}
	for i, label := range labels {
		indicator := "  "
		if i == m.fieldFocus {
			indicator = "> "
		}
		labelStyle := subtitleStyle
		if i == m.fieldFocus {
			labelStyle = lipgloss.NewStyle().Bold(true).Foreground(primaryColor)
		}
		s += fmt.Sprintf("%s%s\n  %s\n\n", indicator, labelStyle.Render(label), m.fields[i].View())
	}

	if m.err != nil {
		s += lipgloss.NewStyle().Foreground(errorColor).
			Render(fmt.Sprintf("  Error: %v", m.err)) + "\n\n"
	}

	s += helpStyle.Render("  tab/shift+tab: navigate fields  ctrl+s: save  enter: next/save  esc: back")

	return s
}

func (m *EntriesModel) renderEntry(entry *domain.TimeEntry, selected bool) string {
	// Lock indicator
	lock := "  "
	if entry.IsLocked() {
		lock = "🔒"
	}

	date := entry.StartTime.Format("Jan 2")
	clientName := truncateStr(m.clientNames[entry.ClientID], 20)
	hours := formatHours(entry.Duration().Hours())
	amount := formatMoney(entry.Amount())
	desc := truncateStr(entry.Description, 35)

	line := fmt.Sprintf("%s %-7s  %-20s  %6s  %10s  %s",
		lock, date, clientName, hours, amount, desc,
	)

	if selected {
		return "  " + selectedStyle.Render(line)
	}
	if !entry.IsBillable {
		return "  " + lipgloss.NewStyle().Foreground(mutedColor).Render(line)
	}
	return "  " + line
}

func (m *EntriesModel) calcTotals() (float64, float64) {
	var totalHours, totalValue float64
	for _, entry := range m.entries {
		totalHours += entry.Duration().Hours()
		totalValue += entry.Amount()
	}
	return totalHours, totalValue
}
