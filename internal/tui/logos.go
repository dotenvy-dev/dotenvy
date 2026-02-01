package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/dotenvy-dev/dotenvy/internal/detect"
)

// Brand colors (256-color palette approximations)
var providerColors = map[string]lipgloss.Color{
	"vercel":   lipgloss.Color("252"),
	"convex":   lipgloss.Color("208"),
	"railway":  lipgloss.Color("141"),
	"render":   lipgloss.Color("87"),
	"supabase": lipgloss.Color("42"),
	"netlify":  lipgloss.Color("44"),
	"flyio":    lipgloss.Color("135"),
	"dotenv":   lipgloss.Color("214"),
}

// ProviderLogo returns a small ASCII logo for the given provider name.
func ProviderLogo(name string) string {
	switch name {
	case "vercel":
		return "  ▲\n ╱ ╲"
	case "convex":
		return " ╭─╮\n ╰─╯"
	case "railway":
		return " ║═║\n ║═║"
	case "render":
		return " ╭─┐\n ╰─╯"
	case "supabase":
		return "  ⚡\n  ║"
	case "netlify":
		return " ◆──\n ──◆"
	case "flyio":
		return " ╱▔╲\n ╲_╱"
	case "dotenv":
		return " ●──\n ───"
	default:
		return " ■\n ■"
	}
}

// ProviderLogoStyled returns the logo with brand-appropriate colors applied.
func ProviderLogoStyled(name string) string {
	logo := ProviderLogo(name)
	color, ok := providerColors[name]
	if !ok {
		color = lipgloss.Color("252")
	}
	style := lipgloss.NewStyle().Foreground(color)
	return style.Render(logo)
}

// providerDisplayName returns a capitalized display name for a provider.
func providerDisplayName(name string) string {
	switch name {
	case "vercel":
		return "Vercel"
	case "convex":
		return "Convex"
	case "railway":
		return "Railway"
	case "render":
		return "Render"
	case "supabase":
		return "Supabase"
	case "netlify":
		return "Netlify"
	case "flyio":
		return "Fly.io"
	case "dotenv":
		return "dotenv"
	default:
		return name
	}
}

// RenderDetectionSummary renders the full detection summary block that gets
// printed to the terminal before the provider multi-select.
func RenderDetectionSummary(result *detect.Result) string {
	if result == nil || len(result.AllKeys) == 0 {
		return ""
	}

	var b strings.Builder

	// Header line
	sourceLabel := result.SourceFile
	if sourceLabel == "" {
		sourceLabel = "input"
	}
	headerStyle := lipgloss.NewStyle().Foreground(Muted)
	b.WriteString(headerStyle.Render(fmt.Sprintf("  Scanned %s — %d keys found", sourceLabel, len(result.AllKeys))))
	b.WriteString("\n\n")

	if len(result.Providers) == 0 {
		mutedStyle := lipgloss.NewStyle().Foreground(Muted)
		b.WriteString(mutedStyle.Render("  No providers detected from key names"))
		b.WriteString("\n")
		return b.String()
	}

	// Group matches by provider
	providerKeys := make(map[string][]detect.Match)
	for _, m := range result.Matches {
		providerKeys[m.ProviderName] = append(providerKeys[m.ProviderName], m)
	}

	nameStyle := lipgloss.NewStyle().Bold(true)
	separatorStyle := lipgloss.NewStyle().Foreground(Muted)
	weakStyle := lipgloss.NewStyle().Foreground(Muted).Italic(true)

	for _, prov := range result.Providers {
		matches := providerKeys[prov]
		color, ok := providerColors[prov]
		if !ok {
			color = lipgloss.Color("252")
		}
		provStyle := lipgloss.NewStyle().Foreground(color)

		// Logo (two lines) — render side-by-side with info
		logo := ProviderLogo(prov)
		logoLines := strings.Split(logo, "\n")

		displayName := providerDisplayName(prov)
		separator := strings.Repeat("─", len(displayName))

		// Build key list: show up to 3 key names, then "+ N more"
		var keyParts []string
		allStrong := true
		for _, m := range matches {
			if m.Confidence != "strong" {
				allStrong = false
			}
		}
		maxShow := 3
		for i, m := range matches {
			if i >= maxShow {
				break
			}
			keyParts = append(keyParts, m.Key)
		}
		keysStr := strings.Join(keyParts, ", ")
		if len(matches) > maxShow {
			keysStr += fmt.Sprintf(" + %d more", len(matches)-maxShow)
		}

		// Line 1: logo top | provider name | keys
		line1Logo := provStyle.Render(logoLines[0])
		line1Info := nameStyle.Render(providerDisplayName(prov)) + "       " + keysStr

		// Line 2: logo bottom | separator | weak note
		line2Logo := provStyle.Render(logoLines[1])
		line2Info := separatorStyle.Render(separator)
		if !allStrong {
			line2Info += "  " + weakStyle.Render("(inferred)")
		}

		// Pad logo to consistent width for alignment
		padWidth := 10
		b.WriteString(fmt.Sprintf("%-*s%s\n", padWidth, line1Logo, line1Info))
		b.WriteString(fmt.Sprintf("%-*s%s\n", padWidth, line2Logo, line2Info))
		b.WriteString("\n")
	}

	// Footer summary
	footerStyle := lipgloss.NewStyle().Foreground(Muted)
	provCount := len(result.Providers)
	keyCount := len(result.AllKeys)
	provWord := "providers"
	if provCount == 1 {
		provWord = "provider"
	}
	secretWord := "secrets"
	if keyCount == 1 {
		secretWord = "secret"
	}
	b.WriteString(footerStyle.Render(fmt.Sprintf("  %d %s detected · %d %s will be imported", provCount, provWord, keyCount, secretWord)))
	b.WriteString("\n")

	return b.String()
}
