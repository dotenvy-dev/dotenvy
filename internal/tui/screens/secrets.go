package screens

import (
	"fmt"
	"strings"

	"github.com/dotenvy-dev/dotenvy/internal/tui"
)

// SecretsView renders the secrets list screen
type SecretsView struct {
	SecretNames []string
	Cursor      int
	Width       int
	Height      int
}

// Render returns the secrets list view
func (s SecretsView) Render() string {
	var b strings.Builder

	// Header
	b.WriteString(tui.TitleStyle.Render("Secret Schema"))
	b.WriteString("\n\n")

	if len(s.SecretNames) == 0 {
		b.WriteString(tui.MutedTextStyle.Render("No secrets in schema."))
		b.WriteString("\n\n")
		b.WriteString("Press 'a' to add a secret name.")
		return b.String()
	}

	b.WriteString(tui.MutedTextStyle.Render("Secret names tracked by dotenvy:"))
	b.WriteString("\n\n")

	// Secrets list
	maxDisplay := s.Height - 10
	if maxDisplay < 5 {
		maxDisplay = 5
	}

	start := 0
	if s.Cursor >= maxDisplay {
		start = s.Cursor - maxDisplay + 1
	}

	for i := start; i < len(s.SecretNames) && i < start+maxDisplay; i++ {
		name := s.SecretNames[i]
		style := tui.NormalStyle
		prefix := "  "
		if i == s.Cursor {
			style = tui.SelectedStyle
			prefix = "> "
		}

		line := fmt.Sprintf("%s%s", prefix, name)
		b.WriteString(style.Render(line))
		b.WriteString("\n")
	}

	// Scroll indicator
	if len(s.SecretNames) > maxDisplay {
		b.WriteString(tui.MutedTextStyle.Render(fmt.Sprintf("\n  ... %d/%d", s.Cursor+1, len(s.SecretNames))))
	}

	// Footer
	b.WriteString("\n\n")
	controls := []string{
		tui.RenderKeyHint("a", "add"),
		tui.RenderKeyHint("d", "delete"),
		tui.RenderKeyHint("esc", "back"),
	}
	b.WriteString(strings.Join(controls, "  "))

	return b.String()
}
