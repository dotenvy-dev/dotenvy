package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	Primary   = lipgloss.Color("212")
	Secondary = lipgloss.Color("99")
	Success   = lipgloss.Color("42")
	Warning   = lipgloss.Color("214")
	Error     = lipgloss.Color("196")
	Muted     = lipgloss.Color("240")
	Subtle    = lipgloss.Color("238")

	// Base styles
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Primary).
			Padding(0, 1)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(Secondary)

	// Box styles
	BoxStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(Subtle).
			Padding(0, 1)

	ActiveBoxStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(Primary).
			Padding(0, 1)

	// Item styles
	SelectedStyle = lipgloss.NewStyle().
			Foreground(Primary).
			Bold(true)

	NormalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	MutedTextStyle = lipgloss.NewStyle().
			Foreground(Muted)

	// Status styles
	SuccessStyle = lipgloss.NewStyle().
			Foreground(Success)

	WarningStyle = lipgloss.NewStyle().
			Foreground(Warning)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(Error)

	InfoStyle = lipgloss.NewStyle().
			Foreground(Secondary)

	// Key hint styles
	KeyStyle = lipgloss.NewStyle().
			Foreground(Primary).
			Bold(true)

	HelpStyle = lipgloss.NewStyle().
			Foreground(Muted)

	// Logo
	LogoStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Primary)
)

// RenderLogo returns the styled logo
func RenderLogo() string {
	return LogoStyle.Render("d o t e n v y")
}

// RenderKeyHint renders a keyboard shortcut hint
func RenderKeyHint(key, description string) string {
	return KeyStyle.Render("["+key+"]") + " " + HelpStyle.Render(description)
}

// RenderStatus renders a status indicator
func RenderStatus(ok bool, label string) string {
	if ok {
		return SuccessStyle.Render("✓") + " " + label
	}
	return ErrorStyle.Render("✗") + " " + label
}

// RenderPendingStatus renders a pending status
func RenderPendingStatus(count int, label string) string {
	return WarningStyle.Render("⚠") + " " + WarningStyle.Render(label)
}
