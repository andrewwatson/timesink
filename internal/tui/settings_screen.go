package tui

import (
	"fmt"
	"strconv"

	"github.com/andy/timesink/internal/app"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type settingsMode int

const (
	settingsModeView settingsMode = iota
	settingsModeEdit
)

// settings form field indices
const (
	settingsFieldOutputDir = iota
	settingsFieldPrefix
	settingsFieldDueDays
	settingsFieldTaxRate
	settingsFieldCount
)

type settingsSavedMsg struct {
	err error
}

// SettingsModel manages the settings screen
type SettingsModel struct {
	app        *app.App
	mode       settingsMode
	fields     []textinput.Model
	fieldFocus int
	err        error
	statusMsg  string
}

// NewSettingsModel creates a new settings screen
func NewSettingsModel(a *app.App) tea.Model {
	return &SettingsModel{
		app:  a,
		mode: settingsModeView,
	}
}

// IsCapturingInput returns true when the edit form is active
func (m *SettingsModel) IsCapturingInput() bool {
	return m.mode == settingsModeEdit
}

func (m *SettingsModel) Init() tea.Cmd {
	return nil
}

func (m *SettingsModel) initForm() {
	m.fields = make([]textinput.Model, settingsFieldCount)
	cfg := m.app.Config.Invoice

	// Output directory
	m.fields[settingsFieldOutputDir] = textinput.New()
	m.fields[settingsFieldOutputDir].Placeholder = "/path/to/invoices"
	m.fields[settingsFieldOutputDir].CharLimit = 256
	m.fields[settingsFieldOutputDir].Width = 60
	m.fields[settingsFieldOutputDir].SetValue(cfg.OutputDir)

	// Number prefix
	m.fields[settingsFieldPrefix] = textinput.New()
	m.fields[settingsFieldPrefix].Placeholder = "INV"
	m.fields[settingsFieldPrefix].CharLimit = 20
	m.fields[settingsFieldPrefix].Width = 20
	m.fields[settingsFieldPrefix].SetValue(cfg.NumberPrefix)

	// Default due days
	m.fields[settingsFieldDueDays] = textinput.New()
	m.fields[settingsFieldDueDays].Placeholder = "30"
	m.fields[settingsFieldDueDays].CharLimit = 5
	m.fields[settingsFieldDueDays].Width = 10
	m.fields[settingsFieldDueDays].SetValue(strconv.Itoa(cfg.DefaultDueDays))

	// Default tax rate (display as percentage)
	m.fields[settingsFieldTaxRate] = textinput.New()
	m.fields[settingsFieldTaxRate].Placeholder = "0.0"
	m.fields[settingsFieldTaxRate].CharLimit = 10
	m.fields[settingsFieldTaxRate].Width = 10
	m.fields[settingsFieldTaxRate].SetValue(fmt.Sprintf("%.2f", cfg.DefaultTaxRate*100))

	m.fieldFocus = settingsFieldOutputDir
	m.fields[settingsFieldOutputDir].Focus()
}

func (m *SettingsModel) saveSettings() tea.Cmd {
	return func() tea.Msg {
		outputDir := m.fields[settingsFieldOutputDir].Value()
		prefix := m.fields[settingsFieldPrefix].Value()
		dueDaysStr := m.fields[settingsFieldDueDays].Value()
		taxRateStr := m.fields[settingsFieldTaxRate].Value()

		if outputDir == "" {
			return settingsSavedMsg{err: fmt.Errorf("output directory is required")}
		}
		if prefix == "" {
			return settingsSavedMsg{err: fmt.Errorf("invoice prefix is required")}
		}

		dueDays, err := strconv.Atoi(dueDaysStr)
		if err != nil || dueDays <= 0 {
			return settingsSavedMsg{err: fmt.Errorf("due days must be a positive number")}
		}

		taxRate, err := strconv.ParseFloat(taxRateStr, 64)
		if err != nil || taxRate < 0 {
			return settingsSavedMsg{err: fmt.Errorf("tax rate must be a non-negative number")}
		}

		// Update config (tax rate stored as decimal)
		m.app.Config.Invoice.OutputDir = outputDir
		m.app.Config.Invoice.NumberPrefix = prefix
		m.app.Config.Invoice.DefaultDueDays = dueDays
		m.app.Config.Invoice.DefaultTaxRate = taxRate / 100

		if err := m.app.SaveConfig(); err != nil {
			return settingsSavedMsg{err: fmt.Errorf("failed to save config: %w", err)}
		}

		return settingsSavedMsg{}
	}
}

func (m *SettingsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.mode == settingsModeEdit {
		return m.updateForm(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		m.err = nil
		switch {
		case msg.String() == "enter":
			m.mode = settingsModeEdit
			m.statusMsg = ""
			m.initForm()
			return m, m.fields[m.fieldFocus].Focus()
		}
	}

	return m, nil
}

func (m *SettingsModel) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case settingsSavedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.mode = settingsModeView
		m.statusMsg = "Settings saved"
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.mode = settingsModeView
			m.err = nil
			return m, nil

		case "tab", "down":
			m.fields[m.fieldFocus].Blur()
			m.fieldFocus = (m.fieldFocus + 1) % settingsFieldCount
			return m, m.fields[m.fieldFocus].Focus()

		case "shift+tab", "up":
			m.fields[m.fieldFocus].Blur()
			m.fieldFocus = (m.fieldFocus - 1 + settingsFieldCount) % settingsFieldCount
			return m, m.fields[m.fieldFocus].Focus()

		case "enter":
			if m.fieldFocus == settingsFieldCount-1 {
				return m, m.saveSettings()
			}
			m.fields[m.fieldFocus].Blur()
			m.fieldFocus++
			return m, m.fields[m.fieldFocus].Focus()

		case "ctrl+s":
			return m, m.saveSettings()
		}
	}

	// Update the focused text input
	var cmd tea.Cmd
	m.fields[m.fieldFocus], cmd = m.fields[m.fieldFocus].Update(msg)
	return m, cmd
}

func (m *SettingsModel) View() string {
	if m.mode == settingsModeEdit {
		return m.viewForm()
	}
	return m.viewSettings()
}

func (m *SettingsModel) viewSettings() string {
	var s string
	s += titleStyle.Render("Settings") + "\n\n"

	if m.statusMsg != "" {
		s += lipgloss.NewStyle().Foreground(successColor).
			Render("  "+m.statusMsg) + "\n\n"
	}

	cfg := m.app.Config.Invoice

	labelStyle := lipgloss.NewStyle().Bold(true).Width(22)
	valueStyle := lipgloss.NewStyle().Foreground(primaryColor)

	s += subtitleStyle.Render("  Invoice Settings") + "\n\n"
	s += fmt.Sprintf("  %s %s\n", labelStyle.Render("Output Directory:"), valueStyle.Render(cfg.OutputDir))
	s += fmt.Sprintf("  %s %s\n", labelStyle.Render("Number Prefix:"), valueStyle.Render(cfg.NumberPrefix))
	s += fmt.Sprintf("  %s %s\n", labelStyle.Render("Default Due Days:"), valueStyle.Render(strconv.Itoa(cfg.DefaultDueDays)))

	taxDisplay := fmt.Sprintf("%.2f%%", cfg.DefaultTaxRate*100)
	s += fmt.Sprintf("  %s %s\n", labelStyle.Render("Default Tax Rate:"), valueStyle.Render(taxDisplay))

	s += "\n" + helpStyle.Render("  enter: edit settings")

	return s
}

func (m *SettingsModel) viewForm() string {
	var s string
	s += titleStyle.Render("Edit Settings") + "\n\n"

	labels := []string{"Output Directory:", "Number Prefix:", "Default Due Days:", "Tax Rate (%):"}
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
