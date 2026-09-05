package tui

import "charm.land/lipgloss/v2"

var (
	docStyle = lipgloss.NewStyle().Margin(1, 2)

	headerStyle   = lipgloss.NewStyle().Background(lipgloss.Color("62")).Foreground(lipgloss.Color("230")).Padding(0, 1)
	cursorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("170")).Bold(true)
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("170"))
	fileStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	promptStyle   = lipgloss.NewStyle().Bold(true)
	errStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	warnStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	okStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	hintStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	spaceTitleStyle = lipgloss.NewStyle().Bold(true).MarginBottom(1)

	confirmStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("230")).
			Background(lipgloss.Color("196")).
			Padding(0, 1)
)
