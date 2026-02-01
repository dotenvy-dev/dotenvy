package screens

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/dotenvy-dev/dotenvy/internal/model"
	"github.com/dotenvy-dev/dotenvy/internal/tui"
	"github.com/dotenvy-dev/dotenvy/pkg/provider"
)

// DashboardView renders the dashboard screen
type DashboardView struct {
	SecretNames  []string
	Targets      []model.Target
	SelectedPane int // 0 = secrets, 1 = targets
	Cursor       int
	Width        int
	Height       int
}

// Render returns the dashboard view
func (d DashboardView) Render() string {
	var b strings.Builder

	// Logo/Header
	logoBox := tui.BoxStyle.Width(d.Width - 4).Align(lipgloss.Center).Render(tui.RenderLogo())
	b.WriteString(logoBox)
	b.WriteString("\n\n")

	// Two-column layout
	colWidth := (d.Width - 6) / 2
	if colWidth < 30 {
		colWidth = 30
	}

	// Secrets panel
	secretsContent := d.renderSecretsPanel()
	secretsBox := tui.BoxStyle
	if d.SelectedPane == 0 {
		secretsBox = tui.ActiveBoxStyle
	}
	secretsPanel := secretsBox.Width(colWidth).Render(secretsContent)

	// Targets panel
	targetsContent := d.renderTargetsPanel()
	targetsBox := tui.BoxStyle
	if d.SelectedPane == 1 {
		targetsBox = tui.ActiveBoxStyle
	}
	targetsPanel := targetsBox.Width(colWidth).Render(targetsContent)

	// Join panels horizontally
	panels := lipgloss.JoinHorizontal(lipgloss.Top, secretsPanel, " ", targetsPanel)
	b.WriteString(panels)

	return b.String()
}

func (d DashboardView) renderSecretsPanel() string {
	var b strings.Builder
	b.WriteString(tui.SubtitleStyle.Render("Schema"))
	b.WriteString("\n")

	if len(d.SecretNames) == 0 {
		b.WriteString(tui.MutedTextStyle.Render("No secrets in schema"))
		return b.String()
	}

	maxDisplay := 10
	for i, name := range d.SecretNames {
		if i >= maxDisplay {
			b.WriteString(tui.MutedTextStyle.Render(fmt.Sprintf("... and %d more", len(d.SecretNames)-maxDisplay)))
			break
		}

		style := tui.NormalStyle
		prefix := "  "
		if d.SelectedPane == 0 && i == d.Cursor {
			style = tui.SelectedStyle
			prefix = "> "
		}

		line := fmt.Sprintf("%s%s", prefix, name)
		b.WriteString(style.Render(line))
		b.WriteString("\n")
	}

	return b.String()
}

func (d DashboardView) renderTargetsPanel() string {
	var b strings.Builder
	b.WriteString(tui.SubtitleStyle.Render("Targets"))
	b.WriteString("\n")

	if len(d.Targets) == 0 {
		b.WriteString(tui.MutedTextStyle.Render("No targets configured"))
		return b.String()
	}

	for i, t := range d.Targets {
		style := tui.NormalStyle
		prefix := "  "
		if d.SelectedPane == 1 && i == d.Cursor {
			style = tui.SelectedStyle
			prefix = "> "
		}

		project := t.GetProject()
		if project == "" {
			project = "-"
		}

		var status string
		if t.Type == "dotenv" {
			status = tui.MutedTextStyle.Render("(file)")
		} else {
			status = tui.SuccessStyle.Render("✓")
		}
		if provider.IsWriteOnly(t.Type) {
			status += " " + tui.MutedTextStyle.Render("(w)")
		}

		line := fmt.Sprintf("%s%-12s %-15s %s", prefix, t.Name, project, status)
		b.WriteString(style.Render(line))
		b.WriteString("\n")
	}

	return b.String()
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
