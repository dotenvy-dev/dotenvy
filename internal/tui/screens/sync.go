package screens

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/dotenvy-dev/dotenvy/internal/model"
	"github.com/dotenvy-dev/dotenvy/internal/tui"
)

// SyncView renders the sync preview screen
type SyncView struct {
	Diffs       []model.TargetDiff
	CurrentIdx  int
	Width       int
	Height      int
	IsApplying  bool
	ApplyStatus string
}

// Render returns the sync preview view
func (s SyncView) Render() string {
	var b strings.Builder

	// Header
	header := fmt.Sprintf("Sync Preview                              %d/%d", s.CurrentIdx+1, len(s.Diffs))
	b.WriteString(tui.TitleStyle.Render(header))
	b.WriteString("\n\n")

	if len(s.Diffs) == 0 {
		b.WriteString(tui.MutedTextStyle.Render("No changes to sync"))
		return b.String()
	}

	// Current diff
	diff := s.Diffs[s.CurrentIdx]
	b.WriteString(s.renderDiff(diff))

	// Footer with controls
	b.WriteString("\n")
	if s.IsApplying {
		b.WriteString(tui.WarningStyle.Render("Applying changes..."))
		if s.ApplyStatus != "" {
			b.WriteString("\n")
			b.WriteString(s.ApplyStatus)
		}
	} else {
		controls := []string{
			tui.RenderKeyHint("enter", "apply"),
			tui.RenderKeyHint("esc", "cancel"),
		}
		if len(s.Diffs) > 1 {
			controls = append(controls, tui.RenderKeyHint("→", "next target"))
		}
		b.WriteString(strings.Join(controls, "  "))
	}

	return b.String()
}

func (s SyncView) renderDiff(diff model.TargetDiff) string {
	var b strings.Builder

	// Target header
	title := fmt.Sprintf("%s (%s)", diff.TargetName, diff.Project)
	header := tui.BoxStyle.Width(s.Width - 4).Render(tui.SubtitleStyle.Render(title))
	b.WriteString(header)
	b.WriteString("\n")

	if !diff.HasChanges() {
		b.WriteString(tui.MutedTextStyle.Render("  No changes"))
		return b.String()
	}

	// Group by environment
	grouped := diff.GroupByEnvironment()
	for env, diffs := range grouped {
		b.WriteString(fmt.Sprintf("  %s:\n", env))

		counts := make(map[model.DiffType]int)
		for _, d := range diffs {
			counts[d.Type]++
			if d.Type != model.DiffUnchanged {
				b.WriteString(s.renderSecretDiff(d))
			}
		}

		if counts[model.DiffUnchanged] > 0 {
			b.WriteString(tui.MutedTextStyle.Render(fmt.Sprintf("    = %d unchanged\n", counts[model.DiffUnchanged])))
		}
	}

	return b.String()
}

func (s SyncView) renderSecretDiff(d model.SecretDiff) string {
	var prefix string
	var style lipgloss.Style

	switch d.Type {
	case model.DiffAdd:
		prefix = "+"
		style = tui.SuccessStyle
	case model.DiffChange:
		prefix = "~"
		style = tui.WarningStyle
	case model.DiffRemove:
		prefix = "-"
		style = tui.ErrorStyle
	case model.DiffUnknown:
		prefix = "?"
		style = tui.InfoStyle
	default:
		return ""
	}

	label := diffTypeLabel(d.Type)

	return fmt.Sprintf("    %s %s %s\n",
		style.Render(prefix),
		d.Name,
		style.Render("("+label+")"))
}

func diffTypeLabel(t model.DiffType) string {
	switch t {
	case model.DiffAdd:
		return "new"
	case model.DiffChange:
		return "changed"
	case model.DiffRemove:
		return "removed"
	case model.DiffUnknown:
		return "unknown"
	default:
		return ""
	}
}
