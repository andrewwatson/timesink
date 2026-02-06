package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	primaryColor = lipgloss.Color("39")  // Blue
	accentColor  = lipgloss.Color("205") // Pink
	mutedColor   = lipgloss.Color("241") // Gray
	successColor = lipgloss.Color("76")  // Green
	warningColor = lipgloss.Color("214") // Orange
	errorColor   = lipgloss.Color("196") // Red

	// Base styles
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(primaryColor)
	subtitleStyle = lipgloss.NewStyle().Foreground(mutedColor)
	selectedStyle = lipgloss.NewStyle().Bold(true).Background(primaryColor).Foreground(lipgloss.Color("0"))

	// Box styles
	boxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1)

	// Header/Footer
	headerStyle = lipgloss.NewStyle().Bold(true).Padding(0, 1)
	footerStyle = lipgloss.NewStyle().Foreground(mutedColor)

	// Timer specific
	timerRunningStyle = lipgloss.NewStyle().Bold(true).Foreground(successColor)
	timerPausedStyle  = lipgloss.NewStyle().Bold(true).Foreground(warningColor)
	timerValueStyle   = lipgloss.NewStyle().Foreground(accentColor)
)
