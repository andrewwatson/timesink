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
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("117")) // Bright cyan
	selectedStyle = lipgloss.NewStyle().Bold(true).Background(primaryColor).Foreground(lipgloss.Color("0"))

	// Box styles
	boxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1)

	// Layout
	borderColor    = lipgloss.Color("63") // Soft purple
	appBorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Padding(1, 2)

	// Header/Footer
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(primaryColor).Padding(0, 1)
	footerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("226")).Bold(true) // Bright yellow

	// Timer specific
	timerRunningStyle = lipgloss.NewStyle().Bold(true).Foreground(successColor)
	timerPausedStyle  = lipgloss.NewStyle().Bold(true).Foreground(warningColor)
	timerValueStyle   = lipgloss.NewStyle().Foreground(accentColor)
)
