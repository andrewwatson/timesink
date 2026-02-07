package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/andy/timesink/internal/app"
	"github.com/andy/timesink/internal/domain"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type invoiceViewMode int

const (
	invoiceViewList          invoiceViewMode = iota
	invoiceViewDetail                        // Viewing a single invoice
	invoiceViewGenPickClient                 // Step 1: pick client
	invoiceViewGenPreview                    // Step 2: preview entries
	invoiceViewGenSavePath                   // Step 3: choose save path
)

// InvoicesModel displays invoices in list and detail views
type InvoicesModel struct {
	app       *app.App
	mode      invoiceViewMode
	invoices  []*domain.Invoice
	cursor    int
	selected  *domain.Invoice
	lineItems []*domain.InvoiceLineItem
	loading   bool
	err       error
	statusMsg string

	// Invoice generation state
	genClients   []*domain.Client
	genCursor    int
	genClient    *domain.Client
	genEntries   []*domain.TimeEntry
	savePathInput textinput.Model
}

// IsCapturingInput returns true when the save path input is active
func (m *InvoicesModel) IsCapturingInput() bool {
	return m.mode == invoiceViewGenSavePath
}

type invoicesDataMsg struct {
	invoices []*domain.Invoice
	err      error
}

type invoiceDetailMsg struct {
	invoice   *domain.Invoice
	lineItems []*domain.InvoiceLineItem
	err       error
}

// genClientsMsg carries clients that have unbilled time
type genClientsMsg struct {
	clients []*domain.Client
	err     error
}

// genEntriesMsg carries unbilled entries for a selected client
type genEntriesMsg struct {
	entries []*domain.TimeEntry
	err     error
}

// genDoneMsg signals invoice generation completed
type genDoneMsg struct {
	invoice  *domain.Invoice
	filePath string
	err      error
}

// NewInvoicesModel creates a new invoices screen model
func NewInvoicesModel(a *app.App) tea.Model {
	return &InvoicesModel{
		app:     a,
		mode:    invoiceViewList,
		loading: true,
	}
}

func (m *InvoicesModel) Init() tea.Cmd {
	return m.loadInvoices()
}

func (m *InvoicesModel) loadInvoices() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		invoices, err := m.app.InvoiceService.ListInvoices(ctx, nil, nil)
		if err != nil {
			return invoicesDataMsg{err: err}
		}

		// Resolve client names
		for _, inv := range invoices {
			if inv.ClientID > 0 && inv.Client == nil {
				client, err := m.app.ClientRepo.GetByID(ctx, inv.ClientID)
				if err == nil {
					inv.Client = client
				}
			}
		}

		return invoicesDataMsg{invoices: invoices}
	}
}

func (m *InvoicesModel) loadDetail(id int64) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		invoice, err := m.app.InvoiceService.GetInvoice(ctx, id)
		if err != nil {
			return invoiceDetailMsg{err: err}
		}

		lineItems, err := m.app.InvoiceRepo.GetLineItems(ctx, id)
		if err != nil {
			return invoiceDetailMsg{err: err}
		}

		if invoice.Client == nil && invoice.ClientID > 0 {
			client, err := m.app.ClientRepo.GetByID(ctx, invoice.ClientID)
			if err == nil {
				invoice.Client = client
			}
		}

		return invoiceDetailMsg{invoice: invoice, lineItems: lineItems}
	}
}

// loadGenClients loads active clients that have unbilled time entries
func (m *InvoicesModel) loadGenClients() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		allClients, err := m.app.ClientRepo.List(ctx, false)
		if err != nil {
			return genClientsMsg{err: err}
		}

		// Filter to clients with unbilled entries
		start := time.Now().AddDate(-10, 0, 0)
		end := time.Now()

		var withUnbilled []*domain.Client
		for _, client := range allClients {
			entries, err := m.app.EntryRepo.GetUnbilledByClient(ctx, client.ID, start, end)
			if err != nil {
				continue
			}
			if len(entries) > 0 {
				withUnbilled = append(withUnbilled, client)
			}
		}

		return genClientsMsg{clients: withUnbilled}
	}
}

// loadGenEntries loads unbilled entries for the selected client
func (m *InvoicesModel) loadGenEntries() tea.Cmd {
	clientID := m.genClient.ID
	return func() tea.Msg {
		ctx := context.Background()
		start := time.Now().AddDate(-10, 0, 0)
		end := time.Now()

		entries, err := m.app.EntryRepo.GetUnbilledByClient(ctx, clientID, start, end)
		if err != nil {
			return genEntriesMsg{err: err}
		}
		return genEntriesMsg{entries: entries}
	}
}

// generateInvoice creates draft, adds entries, calculates totals, finalizes, and exports .txt
func (m *InvoicesModel) generateInvoice() tea.Cmd {
	client := m.genClient
	entries := m.genEntries
	a := m.app
	savePath := m.savePathInput.Value()

	return func() tea.Msg {
		ctx := context.Background()

		if len(entries) == 0 {
			return genDoneMsg{err: fmt.Errorf("no entries to invoice")}
		}

		// Determine period from entries
		periodStart := entries[0].StartTime
		periodEnd := entries[0].StartTime
		for _, e := range entries {
			if e.StartTime.Before(periodStart) {
				periodStart = e.StartTime
			}
			if e.StartTime.After(periodEnd) {
				periodEnd = e.StartTime
			}
		}
		// Set period end to end of the last day
		periodEnd = time.Date(periodEnd.Year(), periodEnd.Month(), periodEnd.Day(),
			23, 59, 59, 0, periodEnd.Location())

		// 1. Create draft
		prefix := a.Config.Invoice.NumberPrefix
		if prefix == "" {
			prefix = "INV"
		}
		invoice, err := a.InvoiceService.CreateDraft(ctx, client.ID, periodStart, periodEnd, prefix)
		if err != nil {
			return genDoneMsg{err: fmt.Errorf("create draft: %w", err)}
		}

		// 2. Add entries
		entryIDs := make([]int64, len(entries))
		for i, e := range entries {
			entryIDs[i] = e.ID
		}
		if err := a.InvoiceService.AddEntriesToInvoice(ctx, invoice.ID, entryIDs); err != nil {
			return genDoneMsg{err: fmt.Errorf("add entries: %w", err)}
		}

		// 3. Calculate totals
		taxRate := a.Config.Invoice.DefaultTaxRate
		if err := a.InvoiceService.CalculateTotals(ctx, invoice.ID, taxRate); err != nil {
			return genDoneMsg{err: fmt.Errorf("calculate totals: %w", err)}
		}

		// 4. Finalize (locks entries)
		if err := a.InvoiceService.Finalize(ctx, invoice.ID); err != nil {
			return genDoneMsg{err: fmt.Errorf("finalize: %w", err)}
		}

		// Reload invoice for final totals
		invoice, err = a.InvoiceService.GetInvoice(ctx, invoice.ID)
		if err != nil {
			return genDoneMsg{err: fmt.Errorf("reload invoice: %w", err)}
		}
		invoice.Client = client

		// Set due date
		dueDays := a.Config.Invoice.DefaultDueDays
		if dueDays <= 0 {
			dueDays = 30
		}
		dueDate := time.Now().AddDate(0, 0, dueDays)
		invoice.DueDate = &dueDate

		// Load line items for the .txt
		lineItems, err := a.InvoiceRepo.GetLineItems(ctx, invoice.ID)
		if err != nil {
			return genDoneMsg{err: fmt.Errorf("load line items: %w", err)}
		}

		// 5. Generate .txt file — replace placeholder in save path with real invoice number
		finalPath := strings.Replace(savePath, fmt.Sprintf("%s-%d-xxx.txt", prefix, time.Now().Year()), invoice.InvoiceNumber+".txt", 1)
		if finalPath == savePath && !strings.HasSuffix(finalPath, ".txt") {
			// User typed a directory — append the invoice filename
			finalPath = filepath.Join(finalPath, invoice.InvoiceNumber+".txt")
		}
		filePath, err := writeInvoiceTxt(a, invoice, lineItems, finalPath)
		if err != nil {
			return genDoneMsg{err: fmt.Errorf("write txt: %w", err)}
		}

		return genDoneMsg{invoice: invoice, filePath: filePath}
	}
}

// writeInvoiceTxt writes a formatted text invoice to the given file path
func writeInvoiceTxt(a *app.App, inv *domain.Invoice, items []*domain.InvoiceLineItem, filePath string) (string, error) {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}
	var b strings.Builder

	sep := strings.Repeat("=", 56)
	line := strings.Repeat("-", 56)

	b.WriteString("INVOICE\n")
	b.WriteString(sep + "\n")
	b.WriteString(fmt.Sprintf("Invoice #:  %s\n", inv.InvoiceNumber))
	b.WriteString(fmt.Sprintf("Date:       %s\n", time.Now().Format("Jan 02, 2006")))
	if inv.DueDate != nil {
		b.WriteString(fmt.Sprintf("Due:        %s\n", inv.DueDate.Format("Jan 02, 2006")))
	}

	// From section (user info)
	user := a.Config.User
	if user.Name != "" || user.Email != "" {
		b.WriteString("\nFrom:\n")
		if user.Name != "" {
			b.WriteString(fmt.Sprintf("  %s\n", user.Name))
		}
		if user.Email != "" {
			b.WriteString(fmt.Sprintf("  %s\n", user.Email))
		}
		if user.Address != "" {
			b.WriteString(fmt.Sprintf("  %s\n", user.Address))
		}
		if user.Phone != "" {
			b.WriteString(fmt.Sprintf("  %s\n", user.Phone))
		}
	}

	// Bill To section
	b.WriteString("\nBill To:\n")
	if inv.Client != nil {
		b.WriteString(fmt.Sprintf("  %s\n", inv.Client.Name))
		if inv.Client.Email != "" {
			b.WriteString(fmt.Sprintf("  %s\n", inv.Client.Email))
		}
	}

	b.WriteString("\n" + line + "\n")
	b.WriteString(fmt.Sprintf("%-12s %-24s %8s %10s\n", "Date", "Description", "Hours", "Amount"))
	b.WriteString(line + "\n")

	for _, item := range items {
		desc := item.Description
		if len(desc) > 24 {
			desc = desc[:21] + "..."
		}
		b.WriteString(fmt.Sprintf("%-12s %-24s %8s %10s\n",
			item.Date.Format("Jan 02"),
			desc,
			formatHours(item.Hours),
			formatMoney(item.Amount),
		))
	}

	b.WriteString(line + "\n")
	b.WriteString(fmt.Sprintf("%46s %10s\n", "Subtotal", formatMoney(inv.Subtotal)))
	if inv.TaxRate > 0 {
		b.WriteString(fmt.Sprintf("%38s (%.1f%%) %10s\n", "Tax", inv.TaxRate*100, formatMoney(inv.TaxAmount)))
	} else {
		b.WriteString(fmt.Sprintf("%46s %10s\n", "Tax", formatMoney(inv.TaxAmount)))
	}
	b.WriteString(fmt.Sprintf("%46s %10s\n", "TOTAL", formatMoney(inv.Total)))
	b.WriteString(sep + "\n")

	if err := os.WriteFile(filePath, []byte(b.String()), 0644); err != nil {
		return "", err
	}

	return filePath, nil
}

func (m *InvoicesModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case RefreshDataMsg:
		m.loading = true
		return m, m.loadInvoices()

	case invoicesDataMsg:
		m.loading = false
		m.err = msg.err
		m.invoices = msg.invoices
		return m, nil

	case invoiceDetailMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.selected = msg.invoice
		m.lineItems = msg.lineItems
		m.mode = invoiceViewDetail
		return m, nil

	case genClientsMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			m.mode = invoiceViewList
			return m, nil
		}
		if len(msg.clients) == 0 {
			m.err = fmt.Errorf("no clients with unbilled time")
			m.mode = invoiceViewList
			return m, nil
		}
		m.genClients = msg.clients
		m.genCursor = 0
		m.mode = invoiceViewGenPickClient
		return m, nil

	case genEntriesMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			m.mode = invoiceViewList
			return m, nil
		}
		m.genEntries = msg.entries
		m.mode = invoiceViewGenPreview
		return m, nil

	case genDoneMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			m.mode = invoiceViewList
			return m, nil
		}
		m.statusMsg = fmt.Sprintf("Invoice %s created -> %s", msg.invoice.InvoiceNumber, msg.filePath)
		m.mode = invoiceViewList
		m.genClients = nil
		m.genEntries = nil
		m.genClient = nil
		return m, m.loadInvoices()

	case tea.KeyMsg:
		if m.loading {
			return m, nil
		}

		switch m.mode {
		case invoiceViewList:
			return m.updateList(msg)
		case invoiceViewDetail:
			return m.updateDetail(msg)
		case invoiceViewGenPickClient:
			return m.updateGenPickClient(msg)
		case invoiceViewGenPreview:
			return m.updateGenPreview(msg)
		case invoiceViewGenSavePath:
			return m.updateGenSavePath(msg)
		}
	}

	// Forward all non-key messages to save path input (for cursor blink, etc.)
	if m.mode == invoiceViewGenSavePath {
		var cmd tea.Cmd
		m.savePathInput, cmd = m.savePathInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *InvoicesModel) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.err = nil

	switch {
	case key.Matches(msg, DefaultKeyMap.Up):
		if m.cursor > 0 {
			m.cursor--
		}
	case key.Matches(msg, DefaultKeyMap.Down):
		if m.cursor < len(m.invoices)-1 {
			m.cursor++
		}
	case key.Matches(msg, DefaultKeyMap.Select):
		if len(m.invoices) > 0 {
			m.loading = true
			return m, m.loadDetail(m.invoices[m.cursor].ID)
		}
	case msg.String() == "n":
		m.loading = true
		m.err = nil
		m.statusMsg = ""
		return m, m.loadGenClients()
	}

	return m, nil
}

func (m *InvoicesModel) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, DefaultKeyMap.Back) {
		m.mode = invoiceViewList
		m.selected = nil
		m.lineItems = nil
	}
	return m, nil
}

func (m *InvoicesModel) updateGenPickClient(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, DefaultKeyMap.Back):
		m.mode = invoiceViewList
		m.genClients = nil
		return m, nil
	case key.Matches(msg, DefaultKeyMap.Up):
		if m.genCursor > 0 {
			m.genCursor--
		}
	case key.Matches(msg, DefaultKeyMap.Down):
		if m.genCursor < len(m.genClients)-1 {
			m.genCursor++
		}
	case key.Matches(msg, DefaultKeyMap.Select):
		if len(m.genClients) > 0 {
			m.genClient = m.genClients[m.genCursor]
			m.loading = true
			return m, m.loadGenEntries()
		}
	}
	return m, nil
}

func (m *InvoicesModel) updateGenPreview(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, DefaultKeyMap.Back):
		m.mode = invoiceViewGenPickClient
		m.genEntries = nil
		return m, nil
	case key.Matches(msg, DefaultKeyMap.Select):
		// Initialize save path input with default
		m.savePathInput = textinput.New()
		m.savePathInput.Placeholder = "path/to/invoice.txt"
		m.savePathInput.Width = 60
		m.savePathInput.CharLimit = 256

		outputDir := m.app.Config.Invoice.OutputDir
		if outputDir == "" {
			homeDir, _ := os.UserHomeDir()
			outputDir = filepath.Join(homeDir, ".config", "timesink", "invoices")
		}
		// Use a placeholder name since we don't have the invoice number yet
		prefix := m.app.Config.Invoice.NumberPrefix
		if prefix == "" {
			prefix = "INV"
		}
		defaultPath := filepath.Join(outputDir, fmt.Sprintf("%s-%d-xxx.txt", prefix, time.Now().Year()))
		m.savePathInput.SetValue(defaultPath)

		m.mode = invoiceViewGenSavePath
		return m, m.savePathInput.Focus()
	}
	return m, nil
}

func (m *InvoicesModel) updateGenSavePath(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.mode = invoiceViewGenPreview
			return m, nil
		case "enter":
			savePath := m.savePathInput.Value()
			if savePath == "" {
				m.err = fmt.Errorf("save path cannot be empty")
				return m, nil
			}
			m.loading = true
			return m, m.generateInvoice()
		}
	}

	// Update the text input
	var cmd tea.Cmd
	m.savePathInput, cmd = m.savePathInput.Update(msg)
	return m, cmd
}

func (m *InvoicesModel) View() string {
	if m.loading {
		return "Loading..."
	}

	switch m.mode {
	case invoiceViewDetail:
		return m.viewDetail()
	case invoiceViewGenPickClient:
		return m.viewGenPickClient()
	case invoiceViewGenPreview:
		return m.viewGenPreview()
	case invoiceViewGenSavePath:
		return m.viewGenSavePath()
	default:
		return m.viewList()
	}
}

func (m *InvoicesModel) viewList() string {
	var s string
	s += titleStyle.Render("Invoices") + "\n\n"

	if m.statusMsg != "" {
		s += lipgloss.NewStyle().Foreground(successColor).
			Render("  "+m.statusMsg) + "\n\n"
	}

	if m.err != nil {
		s += lipgloss.NewStyle().Foreground(errorColor).
			Render(fmt.Sprintf("  Error: %v", m.err)) + "\n\n"
	}

	if len(m.invoices) == 0 && m.err == nil {
		s += subtitleStyle.Render("  No invoices yet. Press 'n' to generate one.")
		return s
	}

	// Header
	s += subtitleStyle.Render(fmt.Sprintf(
		"  %-14s  %-20s  %-22s  %10s  %s",
		"Number", "Client", "Period", "Total", "Status",
	)) + "\n"

	for i, inv := range m.invoices {
		clientName := "Unknown"
		if inv.Client != nil {
			clientName = inv.Client.Name
		}

		period := fmt.Sprintf("%s - %s",
			inv.PeriodStart.Format("Jan 02"),
			inv.PeriodEnd.Format("Jan 02, 2006"),
		)

		invLine := fmt.Sprintf("  %-14s  %-20s  %-22s  %10s  %s",
			inv.InvoiceNumber,
			truncateStr(clientName, 20),
			period,
			formatMoney(inv.Total),
			statusBadge(inv.Status),
		)

		if i == m.cursor {
			s += selectedStyle.Render(invLine) + "\n"
		} else {
			s += invLine + "\n"
		}
	}

	s += "\n" + helpStyle.Render("  j/k: navigate  enter: view detail  n: new invoice")

	return s
}

func (m *InvoicesModel) viewDetail() string {
	inv := m.selected
	if inv == nil {
		return "No invoice selected"
	}

	var s string

	clientName := "Unknown"
	if inv.Client != nil {
		clientName = inv.Client.Name
	}

	// Header
	s += titleStyle.Render(fmt.Sprintf("Invoice %s", inv.InvoiceNumber)) + "\n\n"
	s += fmt.Sprintf("  Client:   %s\n", clientName)
	s += fmt.Sprintf("  Period:   %s - %s\n",
		inv.PeriodStart.Format("Jan 02, 2006"),
		inv.PeriodEnd.Format("Jan 02, 2006"),
	)
	if inv.DueDate != nil {
		s += fmt.Sprintf("  Due:      %s\n", inv.DueDate.Format("Jan 02, 2006"))
	}
	s += fmt.Sprintf("  Status:   %s\n", statusBadge(inv.Status))
	s += "\n"

	// Line items
	if len(m.lineItems) == 0 {
		s += subtitleStyle.Render("  No line items") + "\n"
	} else {
		s += subtitleStyle.Render(fmt.Sprintf(
			"  %-12s  %-35s  %8s  %10s",
			"Date", "Description", "Hours", "Amount",
		)) + "\n"

		for _, item := range m.lineItems {
			s += fmt.Sprintf("  %-12s  %-35s  %8s  %10s\n",
				item.Date.Format("Jan 02"),
				truncateStr(item.Description, 35),
				formatHours(item.Hours),
				formatMoney(item.Amount),
			)
		}
	}

	s += "\n"
	s += fmt.Sprintf("  Subtotal:  %10s\n", formatMoney(inv.Subtotal))
	s += fmt.Sprintf("  Tax:       %10s\n", formatMoney(inv.TaxAmount))
	s += lipgloss.NewStyle().Bold(true).Render(
		fmt.Sprintf("  Total:     %10s", formatMoney(inv.Total)),
	) + "\n"

	s += "\n" + helpStyle.Render("  esc: back to list")

	return s
}

func (m *InvoicesModel) viewGenPickClient() string {
	var s string
	s += titleStyle.Render("New Invoice - Select Client") + "\n\n"

	if len(m.genClients) == 0 {
		s += subtitleStyle.Render("  No clients with unbilled time") + "\n"
		s += "\n" + helpStyle.Render("  esc: back")
		return s
	}

	s += subtitleStyle.Render("  Clients with unbilled time:") + "\n\n"

	for i, client := range m.genClients {
		indicator := "  "
		if i == m.genCursor {
			indicator = "> "
		}

		rate := fmt.Sprintf("$%.0f/hr", client.HourlyRate)
		clientLine := fmt.Sprintf("%s%-25s  %s", indicator, client.Name, rate)

		if i == m.genCursor {
			s += lipgloss.NewStyle().Bold(true).Foreground(primaryColor).Render(clientLine) + "\n"
		} else {
			s += clientLine + "\n"
		}
	}

	s += "\n" + helpStyle.Render("  j/k: navigate  enter: select  esc: cancel")

	return s
}

func (m *InvoicesModel) viewGenPreview() string {
	var s string

	clientName := m.genClient.Name
	s += titleStyle.Render(fmt.Sprintf("New Invoice - %s", clientName)) + "\n\n"

	if len(m.genEntries) == 0 {
		s += subtitleStyle.Render("  No unbilled entries found") + "\n"
		s += "\n" + helpStyle.Render("  esc: back")
		return s
	}

	// Summary
	var totalHours, totalValue float64
	for _, e := range m.genEntries {
		totalHours += e.Duration().Hours()
		totalValue += e.Amount()
	}

	taxRate := m.app.Config.Invoice.DefaultTaxRate
	taxAmount := totalValue * taxRate
	total := totalValue + taxAmount

	s += fmt.Sprintf("  %d entries  |  %s  |  %s\n\n",
		len(m.genEntries), formatHours(totalHours), formatMoney(totalValue))

	// Entry table
	s += subtitleStyle.Render(fmt.Sprintf(
		"  %-10s  %-30s  %8s  %10s",
		"Date", "Description", "Hours", "Amount",
	)) + "\n"

	for _, entry := range m.genEntries {
		desc := entry.Description
		if desc == "" {
			desc = "(no description)"
		}

		s += fmt.Sprintf("  %-10s  %-30s  %8s  %10s\n",
			entry.StartTime.Format("Jan 02"),
			truncateStr(desc, 30),
			formatHours(entry.Duration().Hours()),
			formatMoney(entry.Amount()),
		)
	}

	// Totals
	s += "\n"
	s += fmt.Sprintf("  %42s  %10s\n", "Subtotal:", formatMoney(totalValue))
	if taxRate > 0 {
		s += fmt.Sprintf("  %35s (%.1f%%)  %10s\n", "Tax:", taxRate*100, formatMoney(taxAmount))
	} else {
		s += fmt.Sprintf("  %42s  %10s\n", "Tax:", formatMoney(taxAmount))
	}
	s += lipgloss.NewStyle().Bold(true).Render(
		fmt.Sprintf("  %42s  %10s", "Total:", formatMoney(total)),
	) + "\n"

	s += "\n" + lipgloss.NewStyle().Foreground(warningColor).Render(
		"  Press enter to generate invoice and lock these entries") + "\n"
	s += helpStyle.Render("  esc: back to client selection")

	return s
}

func (m *InvoicesModel) viewGenSavePath() string {
	var s string

	clientName := m.genClient.Name
	s += titleStyle.Render(fmt.Sprintf("New Invoice - %s", clientName)) + "\n\n"

	// Summary
	var totalHours, totalValue float64
	for _, e := range m.genEntries {
		totalHours += e.Duration().Hours()
		totalValue += e.Amount()
	}
	taxRate := m.app.Config.Invoice.DefaultTaxRate
	total := totalValue + (totalValue * taxRate)

	s += fmt.Sprintf("  %d entries  |  %s  |  %s\n\n",
		len(m.genEntries), formatHours(totalHours), formatMoney(total))

	s += lipgloss.NewStyle().Bold(true).Foreground(primaryColor).Render("  Save invoice to:") + "\n"
	s += "  " + m.savePathInput.View() + "\n"

	if m.err != nil {
		s += "\n" + lipgloss.NewStyle().Foreground(errorColor).
			Render(fmt.Sprintf("  Error: %v", m.err)) + "\n"
	}

	s += "\n" + helpStyle.Render("  enter: generate and save  esc: back")

	return s
}

// statusBadge renders an invoice status with color
func statusBadge(status domain.InvoiceStatus) string {
	switch status {
	case domain.InvoiceStatusDraft:
		return lipgloss.NewStyle().Foreground(mutedColor).Render("DRAFT")
	case domain.InvoiceStatusFinalized:
		return lipgloss.NewStyle().Foreground(primaryColor).Render("FINALIZED")
	case domain.InvoiceStatusSent:
		return lipgloss.NewStyle().Foreground(warningColor).Render("SENT")
	case domain.InvoiceStatusPaid:
		return lipgloss.NewStyle().Foreground(successColor).Render("PAID")
	case domain.InvoiceStatusOverdue:
		return lipgloss.NewStyle().Foreground(errorColor).Render("OVERDUE")
	default:
		return string(status)
	}
}
