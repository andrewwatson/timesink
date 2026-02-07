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

// clientMode represents the current screen mode
type clientMode int

const (
	clientModeList clientMode = iota
	clientModeNew
	clientModeEdit
)

// form field indices
const (
	fieldName = iota
	fieldRate
	fieldEmail
	fieldNotes
	fieldCount
)

// ClientsModel displays a navigable list of clients with create/edit forms
type ClientsModel struct {
	app          *app.App
	clients      []*domain.Client
	cursor       int
	showArchived bool
	monthlyStats map[int64]*clientMonthStats
	loading      bool
	err          error
	statusMsg    string

	// Form state
	mode           clientMode
	fields         []textinput.Model
	fieldFocus     int
	editingID      int64 // 0 for new client
	autoNewClient  bool  // open new client form after data loads
}

type clientMonthStats struct {
	hours float64
	value float64
}

type clientsDataMsg struct {
	clients      []*domain.Client
	monthlyStats map[int64]*clientMonthStats
	err          error
}

type clientSavedMsg struct {
	name string
	err  error
}

// NewClientsModel creates a new clients screen model
func NewClientsModel(a *app.App) tea.Model {
	return &ClientsModel{
		app:          a,
		monthlyStats: make(map[int64]*clientMonthStats),
		loading:      true,
	}
}

// IsCapturingInput returns true when the form is active
func (m *ClientsModel) IsCapturingInput() bool {
	return m.mode == clientModeNew || m.mode == clientModeEdit
}

func (m *ClientsModel) Init() tea.Cmd {
	return m.loadClients()
}

func (m *ClientsModel) loadClients() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		clients, err := m.app.ClientRepo.List(ctx, m.showArchived)
		if err != nil {
			return clientsDataMsg{err: err}
		}

		// Calculate monthly stats per client
		now := time.Now()
		monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		monthEnd := monthStart.AddDate(0, 1, 0)

		stats := make(map[int64]*clientMonthStats)
		for _, client := range clients {
			cid := client.ID
			entries, err := m.app.EntryRepo.List(ctx, &cid, &monthStart, &monthEnd, true)
			if err != nil {
				continue
			}
			cs := &clientMonthStats{}
			for _, entry := range entries {
				cs.hours += entry.Duration().Hours()
				cs.value += entry.Amount()
			}
			stats[client.ID] = cs
		}

		return clientsDataMsg{
			clients:      clients,
			monthlyStats: stats,
		}
	}
}

func (m *ClientsModel) initForm(editing *domain.Client) {
	m.fields = make([]textinput.Model, fieldCount)

	// Name field
	m.fields[fieldName] = textinput.New()
	m.fields[fieldName].Placeholder = "Client name"
	m.fields[fieldName].CharLimit = 100
	m.fields[fieldName].Width = 40

	// Rate field
	m.fields[fieldRate] = textinput.New()
	m.fields[fieldRate].Placeholder = "150.00"
	m.fields[fieldRate].CharLimit = 10
	m.fields[fieldRate].Width = 15

	// Email field
	m.fields[fieldEmail] = textinput.New()
	m.fields[fieldEmail].Placeholder = "email@example.com"
	m.fields[fieldEmail].CharLimit = 100
	m.fields[fieldEmail].Width = 40

	// Notes field
	m.fields[fieldNotes] = textinput.New()
	m.fields[fieldNotes].Placeholder = "Optional notes"
	m.fields[fieldNotes].CharLimit = 200
	m.fields[fieldNotes].Width = 50

	// Pre-fill for editing
	if editing != nil {
		m.fields[fieldName].SetValue(editing.Name)
		m.fields[fieldRate].SetValue(fmt.Sprintf("%.2f", editing.HourlyRate))
		m.fields[fieldEmail].SetValue(editing.Email)
		m.fields[fieldNotes].SetValue(editing.Notes)
		m.editingID = editing.ID
	} else {
		m.editingID = 0
	}

	m.fieldFocus = fieldName
	m.fields[fieldName].Focus()
}

func (m *ClientsModel) saveClient() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		name := m.fields[fieldName].Value()
		rateStr := m.fields[fieldRate].Value()
		email := m.fields[fieldEmail].Value()
		notes := m.fields[fieldNotes].Value()

		if name == "" {
			return clientSavedMsg{err: fmt.Errorf("name is required")}
		}

		rate, err := strconv.ParseFloat(rateStr, 64)
		if err != nil && rateStr != "" {
			return clientSavedMsg{err: fmt.Errorf("invalid rate: %s", rateStr)}
		}

		if m.editingID > 0 {
			// Update existing
			client, err := m.app.ClientRepo.GetByID(ctx, m.editingID)
			if err != nil {
				return clientSavedMsg{err: err}
			}
			client.Name = name
			client.HourlyRate = rate
			client.Email = email
			client.Notes = notes
			client.UpdatedAt = time.Now()

			if err := m.app.ClientRepo.Update(ctx, client); err != nil {
				return clientSavedMsg{err: err}
			}
			return clientSavedMsg{name: name}
		}

		// Create new
		client := domain.NewClient(name, rate)
		client.Email = email
		client.Notes = notes

		if err := m.app.ClientRepo.Create(ctx, client); err != nil {
			return clientSavedMsg{err: err}
		}
		return clientSavedMsg{name: name}
	}
}

func (m *ClientsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle OpenNewClientFormMsg at the top so it works regardless of mode
	if _, ok := msg.(OpenNewClientFormMsg); ok {
		if m.loading {
			// Data hasn't loaded yet; set flag to auto-open form when it does
			m.autoNewClient = true
			return m, nil
		}
		m.mode = clientModeNew
		m.initForm(nil)
		return m, m.fields[fieldName].Focus()
	}

	// Handle form mode
	if m.mode == clientModeNew || m.mode == clientModeEdit {
		return m.updateForm(msg)
	}

	switch msg := msg.(type) {
	case RefreshDataMsg:
		m.loading = true
		return m, m.loadClients()

	case clientsDataMsg:
		m.loading = false
		m.err = msg.err
		if msg.err == nil {
			m.clients = msg.clients
			m.monthlyStats = msg.monthlyStats
			if m.cursor >= len(m.clients) {
				m.cursor = max(0, len(m.clients)-1)
			}
		}
		// Auto-open new client form on first run
		if m.autoNewClient {
			m.autoNewClient = false
			m.mode = clientModeNew
			m.initForm(nil)
			return m, m.fields[fieldName].Focus()
		}
		return m, nil

	case clientSavedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.mode = clientModeList
		m.statusMsg = fmt.Sprintf("Saved: %s", msg.name)
		m.loading = true
		return m, m.loadClients()

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
			}
		case key.Matches(msg, DefaultKeyMap.Down):
			if m.cursor < len(m.clients)-1 {
				m.cursor++
			}
		case msg.String() == "n":
			m.mode = clientModeNew
			m.initForm(nil)
			return m, m.fields[fieldName].Focus()
		case key.Matches(msg, DefaultKeyMap.Select):
			// Enter key opens edit form for selected client
			if len(m.clients) > 0 && m.cursor < len(m.clients) {
				m.mode = clientModeEdit
				m.initForm(m.clients[m.cursor])
				return m, m.fields[fieldName].Focus()
			}
		case msg.String() == "a":
			if len(m.clients) > 0 && m.cursor < len(m.clients) {
				return m, m.toggleArchive()
			}
		case msg.String() == "h":
			m.showArchived = !m.showArchived
			m.cursor = 0
			m.loading = true
			return m, m.loadClients()
		}
	}

	return m, nil
}

func (m *ClientsModel) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case clientSavedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.mode = clientModeList
		m.statusMsg = fmt.Sprintf("Saved: %s", msg.name)
		m.loading = true
		return m, m.loadClients()

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			// Cancel form
			m.mode = clientModeList
			m.err = nil
			return m, nil

		case "tab", "down":
			// Next field
			m.fields[m.fieldFocus].Blur()
			m.fieldFocus = (m.fieldFocus + 1) % fieldCount
			return m, m.fields[m.fieldFocus].Focus()

		case "shift+tab", "up":
			// Previous field
			m.fields[m.fieldFocus].Blur()
			m.fieldFocus = (m.fieldFocus - 1 + fieldCount) % fieldCount
			return m, m.fields[m.fieldFocus].Focus()

		case "enter":
			// If on last field or explicit submit, save
			if m.fieldFocus == fieldCount-1 {
				return m, m.saveClient()
			}
			// Otherwise advance to next field
			m.fields[m.fieldFocus].Blur()
			m.fieldFocus++
			return m, m.fields[m.fieldFocus].Focus()

		case "ctrl+s":
			// Save from any field
			return m, m.saveClient()
		}
	}

	// Update the focused text input
	var cmd tea.Cmd
	m.fields[m.fieldFocus], cmd = m.fields[m.fieldFocus].Update(msg)
	return m, cmd
}

func (m *ClientsModel) toggleArchive() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		client := m.clients[m.cursor]

		if client.IsArchived {
			m.app.ClientRepo.Unarchive(ctx, client.ID)
		} else {
			m.app.ClientRepo.Archive(ctx, client.ID)
		}

		// Reload
		return m.loadClients()()
	}
}

func (m *ClientsModel) View() string {
	if m.mode == clientModeNew || m.mode == clientModeEdit {
		return m.viewForm()
	}
	return m.viewList()
}

func (m *ClientsModel) viewForm() string {
	var s string

	if m.mode == clientModeNew {
		if len(m.clients) == 0 {
			s += titleStyle.Render("Welcome to timesink!") + "\n"
			s += subtitleStyle.Render("  Let's set up your first client to get started.") + "\n\n"
		} else {
			s += titleStyle.Render("New Client") + "\n\n"
		}
	} else {
		s += titleStyle.Render("Edit Client") + "\n\n"
	}

	labels := []string{"Name:", "Rate ($/hr):", "Email:", "Notes:"}
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

	s += helpStyle.Render("  tab/shift+tab: navigate fields  ctrl+s: save  enter: next/save  esc: cancel")

	return s
}

func (m *ClientsModel) viewList() string {
	if m.loading {
		return "Loading clients..."
	}

	if m.err != nil {
		return lipgloss.NewStyle().Foreground(errorColor).
			Render(fmt.Sprintf("Error: %v", m.err))
	}

	var s string

	// Header
	header := "Clients"
	if m.showArchived {
		header += subtitleStyle.Render("  (showing archived)")
	}
	s += titleStyle.Render(header) + "\n\n"

	// Status message
	if m.statusMsg != "" {
		s += lipgloss.NewStyle().Foreground(successColor).
			Render("  "+m.statusMsg) + "\n\n"
	}

	if len(m.clients) == 0 {
		s += subtitleStyle.Render("  No clients yet. Press 'n' to add one.") + "\n"
		s += subtitleStyle.Render("  Press 'h' to toggle archived clients") + "\n"
		return s
	}

	for i, client := range m.clients {
		s += m.renderClient(i, client) + "\n"
	}

	s += "\n" + helpStyle.Render("  j/k: navigate  n: new  enter: edit  a: archive/unarchive  h: toggle archived")

	return s
}

func (m *ClientsModel) renderClient(index int, client *domain.Client) string {
	selected := index == m.cursor

	// Name
	name := client.Name
	if client.IsArchived {
		name += " (archived)"
	}

	// Rate
	rate := fmt.Sprintf("$%.0f/hr", client.HourlyRate)

	// Monthly stats
	stats := m.monthlyStats[client.ID]
	hours := 0.0
	value := 0.0
	if stats != nil {
		hours = stats.hours
		value = stats.value
	}
	monthly := fmt.Sprintf("This month: %s  %s", formatHours(hours), formatMoney(value))

	// Contact
	contact := client.Email
	if contact == "" && client.Notes != "" {
		contact = truncateStr(client.Notes, 40)
	}

	// Build row
	indicator := "  "
	if selected {
		indicator = "> "
	}

	line1 := fmt.Sprintf("%s%s", indicator, name)
	line2 := fmt.Sprintf("    Rate: %s  |  %s", rate, monthly)
	var line3 string
	if contact != "" {
		line3 = fmt.Sprintf("    %s", contact)
	}

	// Apply styling
	nameStyle := lipgloss.NewStyle()
	detailStyle := subtitleStyle
	if client.IsArchived {
		nameStyle = nameStyle.Foreground(mutedColor)
		detailStyle = lipgloss.NewStyle().Foreground(mutedColor)
	}
	if selected {
		nameStyle = nameStyle.Bold(true).Foreground(primaryColor)
	}

	result := nameStyle.Render(line1) + "\n" + detailStyle.Render(line2)
	if line3 != "" {
		result += "\n" + detailStyle.Render(line3)
	}

	return result
}
