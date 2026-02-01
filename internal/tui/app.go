package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dotenvy-dev/dotenvy/internal/auth"
	"github.com/dotenvy-dev/dotenvy/internal/config"
	"github.com/dotenvy-dev/dotenvy/internal/model"
	"github.com/dotenvy-dev/dotenvy/internal/source"
	"github.com/dotenvy-dev/dotenvy/internal/sync"
)

// Screen represents different TUI screens
type Screen int

const (
	ScreenDashboard Screen = iota
	ScreenSyncSetup
	ScreenSyncPreview
	ScreenSyncing
)

// KeyMap defines keyboard shortcuts
type KeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Left    key.Binding
	Right   key.Binding
	Enter   key.Binding
	Tab     key.Binding
	Sync    key.Binding
	Refresh key.Binding
	Help    key.Binding
	Quit    key.Binding
	Escape  key.Binding
}

var DefaultKeyMap = KeyMap{
	Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	Left:    key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←", "prev")),
	Right:   key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→", "next")),
	Enter:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
	Tab:     key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch panel")),
	Sync:    key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "sync")),
	Refresh: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	Help:    key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	Escape:  key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
}

// Model is the main TUI model
type Model struct {
	config       *config.Config
	secretNames  []string
	targets      []model.Target
	screen       Screen
	cursor       int
	width        int
	height       int
	keys         KeyMap
	err          error
	configPath   string
	showHelp     bool
	selectedPane int // 0 = secrets, 1 = targets

	// Sync setup
	syncEnv      string
	syncEnvInput textinput.Model
	syncFile     string
	syncFileInput textinput.Model
	setupFocused int // 0 = env, 1 = file

	// Sync state
	diffs          []model.TargetDiff
	currentDiffIdx int
	syncing        bool
	syncSpinner    spinner.Model
	syncStatus     string
	syncResults    *sync.SyncResult
}

// New creates a new TUI model
func New(configPath string) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(Primary)

	envInput := textinput.New()
	envInput.Placeholder = "test or live"
	envInput.Focus()

	fileInput := textinput.New()
	fileInput.Placeholder = ".env.test (optional)"

	return Model{
		keys:          DefaultKeyMap,
		configPath:    configPath,
		syncSpinner:   s,
		syncEnvInput:  envInput,
		syncFileInput: fileInput,
	}
}

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.loadConfig, m.syncSpinner.Tick, textinput.Blink)
}

// Messages
type configLoadedMsg struct {
	config *config.Config
	err    error
}

type diffsCalculatedMsg struct {
	diffs []model.TargetDiff
	err   error
}

type syncCompleteMsg struct {
	result *sync.SyncResult
	err    error
}

func (m Model) loadConfig() tea.Msg {
	cfg, err := config.Load(m.configPath)
	if err != nil {
		return configLoadedMsg{err: err}
	}
	return configLoadedMsg{config: cfg}
}

func (m Model) calculateDiffs() tea.Msg {
	engine := sync.NewEngine()
	ctx := context.Background()

	// Build source
	var src source.Source
	if m.syncFile != "" {
		src = source.NewFileSource(m.syncFile)
	} else {
		src = source.NewEnvSource()
	}

	// Calculate diffs for each target
	var diffs []model.TargetDiff
	for _, target := range m.targets {
		remoteEnvs := target.MapToRemote(m.syncEnv)
		for _, remoteEnv := range remoteEnvs {
			diff, err := engine.Preview(ctx, m.secretNames, src, target, remoteEnv)
			if err != nil {
				return diffsCalculatedMsg{err: err}
			}
			diffs = append(diffs, *diff)
		}
	}

	return diffsCalculatedMsg{diffs: diffs}
}

func (m Model) performSync() tea.Msg {
	engine := sync.NewEngine()
	ctx := context.Background()

	// Build source
	var src source.Source
	if m.syncFile != "" {
		src = source.NewFileSource(m.syncFile)
	} else {
		src = source.NewEnvSource()
	}

	// Sync each diff
	for _, diff := range m.diffs {
		if !diff.HasChanges() {
			continue
		}

		// Find the target
		var target model.Target
		for _, t := range m.targets {
			if t.Name == diff.TargetName {
				target = t
				break
			}
		}

		remoteEnv := ""
		if len(diff.Diffs) > 0 {
			remoteEnv = diff.Diffs[0].Environment
		}

		result, err := engine.Sync(ctx, m.secretNames, src, target, remoteEnv, sync.SyncOptions{})
		if err != nil {
			return syncCompleteMsg{err: err}
		}
		// Accumulate results
		if m.syncResults == nil {
			return syncCompleteMsg{result: result}
		}
	}

	return syncCompleteMsg{result: m.syncResults}
}

// Update implements tea.Model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case configLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.config = msg.config
		m.secretNames = msg.config.GetSecretNames()
		m.targets = msg.config.GetTargets()
		return m, nil

	case diffsCalculatedMsg:
		m.syncing = false
		if msg.err != nil {
			m.err = msg.err
			m.screen = ScreenDashboard
			return m, nil
		}
		m.diffs = msg.diffs
		m.currentDiffIdx = 0
		m.screen = ScreenSyncPreview
		return m, nil

	case syncCompleteMsg:
		m.syncing = false
		if msg.err != nil {
			m.err = msg.err
		}
		m.syncResults = msg.result
		m.screen = ScreenDashboard
		m.syncStatus = "Sync complete!"
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.syncSpinner, cmd = m.syncSpinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		// Clear any error on keypress
		if m.err != nil && msg.String() != "q" && msg.String() != "ctrl+c" {
			m.err = nil
			return m, nil
		}

		return m.handleKeyPress(msg)
	}

	// Handle text input updates in setup screen
	if m.screen == ScreenSyncSetup {
		var cmd tea.Cmd
		if m.setupFocused == 0 {
			m.syncEnvInput, cmd = m.syncEnvInput.Update(msg)
		} else {
			m.syncFileInput, cmd = m.syncFileInput.Update(msg)
		}
		return m, cmd
	}

	return m, nil
}

func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Help):
		m.showHelp = !m.showHelp
		return m, nil

	case key.Matches(msg, m.keys.Escape):
		if m.screen != ScreenDashboard {
			m.screen = ScreenDashboard
			m.err = nil
			m.syncStatus = ""
		}
		return m, nil
	}

	// Screen-specific handling
	switch m.screen {
	case ScreenDashboard:
		return m.handleDashboardKeys(msg)
	case ScreenSyncSetup:
		return m.handleSyncSetupKeys(msg)
	case ScreenSyncPreview:
		return m.handleSyncPreviewKeys(msg)
	}

	return m, nil
}

func (m Model) handleDashboardKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Up):
		if m.cursor > 0 {
			m.cursor--
		}

	case key.Matches(msg, m.keys.Down):
		max := m.maxCursorItems()
		if m.cursor < max-1 {
			m.cursor++
		}

	case key.Matches(msg, m.keys.Tab):
		m.selectedPane = (m.selectedPane + 1) % 2
		m.cursor = 0

	case key.Matches(msg, m.keys.Sync):
		// Check auth first
		allAuth := true
		for _, t := range m.targets {
			if t.Type == "dotenv" {
				continue
			}
			status := auth.CheckAuth(t.Name, t.Type, t.Config)
			if !status.Authenticated {
				allAuth = false
				m.err = fmt.Errorf("not authenticated for %s", t.Name)
				break
			}
		}
		if allAuth {
			m.screen = ScreenSyncSetup
			m.setupFocused = 0
			m.syncEnvInput.Focus()
			m.syncFileInput.Blur()
			return m, textinput.Blink
		}

	case key.Matches(msg, m.keys.Refresh):
		return m, m.loadConfig
	}

	return m, nil
}

func (m Model) handleSyncSetupKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Tab):
		m.setupFocused = (m.setupFocused + 1) % 2
		if m.setupFocused == 0 {
			m.syncEnvInput.Focus()
			m.syncFileInput.Blur()
		} else {
			m.syncEnvInput.Blur()
			m.syncFileInput.Focus()
		}
		return m, textinput.Blink

	case key.Matches(msg, m.keys.Enter):
		m.syncEnv = m.syncEnvInput.Value()
		m.syncFile = m.syncFileInput.Value()

		if m.syncEnv == "" {
			m.err = fmt.Errorf("environment is required (test or live)")
			return m, nil
		}

		m.syncing = true
		m.syncStatus = "Calculating changes..."
		return m, m.calculateDiffs
	}

	return m, nil
}

func (m Model) handleSyncPreviewKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Left):
		if m.currentDiffIdx > 0 {
			m.currentDiffIdx--
		}

	case key.Matches(msg, m.keys.Right):
		if m.currentDiffIdx < len(m.diffs)-1 {
			m.currentDiffIdx++
		}

	case key.Matches(msg, m.keys.Enter):
		// Apply sync
		hasChanges := false
		for _, d := range m.diffs {
			if d.HasChanges() {
				hasChanges = true
				break
			}
		}
		if hasChanges {
			m.syncing = true
			m.screen = ScreenSyncing
			m.syncStatus = "Applying changes..."
			return m, m.performSync
		}
	}

	return m, nil
}

func (m Model) maxCursorItems() int {
	if m.selectedPane == 0 {
		return len(m.secretNames)
	}
	return len(m.targets)
}

// View implements tea.Model
func (m Model) View() string {
	if m.config == nil && m.err == nil {
		return "Loading..."
	}

	var b strings.Builder

	// Error display
	if m.err != nil {
		b.WriteString(ErrorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		b.WriteString("\n\nPress any key to continue, 'q' to quit")
		return b.String()
	}

	switch m.screen {
	case ScreenDashboard:
		return m.renderDashboard()
	case ScreenSyncSetup:
		return m.renderSyncSetup()
	case ScreenSyncPreview:
		return m.renderSyncPreview()
	case ScreenSyncing:
		return m.renderSyncing()
	}

	return m.renderDashboard()
}

func (m Model) renderDashboard() string {
	var b strings.Builder

	// Logo/Header
	logoBox := BoxStyle.Width(min(m.width-4, 60)).Align(lipgloss.Center).Render(RenderLogo())
	b.WriteString(logoBox)
	b.WriteString("\n\n")

	// Status message
	if m.syncStatus != "" {
		b.WriteString(SuccessStyle.Render(m.syncStatus))
		b.WriteString("\n\n")
	}

	// Two-column layout
	colWidth := min((m.width-6)/2, 40)

	// Secrets panel
	secretsContent := m.renderSecretsPanel()
	secretsBox := BoxStyle
	if m.selectedPane == 0 {
		secretsBox = ActiveBoxStyle
	}
	secretsPanel := secretsBox.Width(colWidth).Height(min(len(m.secretNames)+3, 15)).Render(secretsContent)

	// Targets panel
	targetsContent := m.renderTargetsPanel()
	targetsBox := BoxStyle
	if m.selectedPane == 1 {
		targetsBox = ActiveBoxStyle
	}
	targetsPanel := targetsBox.Width(colWidth).Height(min(len(m.targets)+3, 15)).Render(targetsContent)

	// Join panels horizontally
	panels := lipgloss.JoinHorizontal(lipgloss.Top, secretsPanel, " ", targetsPanel)
	b.WriteString(panels)
	b.WriteString("\n\n")

	// Spinner when loading
	if m.syncing {
		b.WriteString(m.syncSpinner.View() + " " + m.syncStatus)
		b.WriteString("\n\n")
	}

	// Help footer
	b.WriteString(m.renderHelp())

	return b.String()
}

func (m Model) renderSecretsPanel() string {
	var b strings.Builder
	b.WriteString(SubtitleStyle.Render("Schema"))
	b.WriteString("\n")

	if len(m.secretNames) == 0 {
		b.WriteString(MutedTextStyle.Render("No secrets in schema"))
		return b.String()
	}

	for i, name := range m.secretNames {
		if i >= 10 {
			b.WriteString(MutedTextStyle.Render(fmt.Sprintf("... +%d more", len(m.secretNames)-10)))
			break
		}

		style := NormalStyle
		prefix := "  "
		if m.selectedPane == 0 && i == m.cursor {
			style = SelectedStyle
			prefix = "> "
		}

		line := fmt.Sprintf("%s%s", prefix, name)
		b.WriteString(style.Render(line))
		b.WriteString("\n")
	}

	return b.String()
}

func (m Model) renderTargetsPanel() string {
	var b strings.Builder
	b.WriteString(SubtitleStyle.Render("Targets"))
	b.WriteString("\n")

	if len(m.targets) == 0 {
		b.WriteString(MutedTextStyle.Render("No targets configured"))
		return b.String()
	}

	for i, t := range m.targets {
		style := NormalStyle
		prefix := "  "
		if m.selectedPane == 1 && i == m.cursor {
			style = SelectedStyle
			prefix = "> "
		}

		// Check auth status
		var statusStr string
		if t.Type == "dotenv" {
			statusStr = MutedTextStyle.Render("(file)")
		} else {
			status := auth.CheckAuth(t.Name, t.Type, t.Config)
			if status.Authenticated {
				statusStr = SuccessStyle.Render("✓")
			} else {
				statusStr = ErrorStyle.Render("✗")
			}
		}

		project := truncate(t.GetProject(), 12)
		if project == "" {
			project = "-"
		}

		line := fmt.Sprintf("%s%-10s %-12s %s", prefix, t.Name, project, statusStr)
		b.WriteString(style.Render(line))
		b.WriteString("\n")
	}

	return b.String()
}

func (m Model) renderSyncSetup() string {
	var b strings.Builder

	b.WriteString(TitleStyle.Render("Sync Setup"))
	b.WriteString("\n\n")

	b.WriteString("Environment (test or live):\n")
	b.WriteString(m.syncEnvInput.View())
	b.WriteString("\n\n")

	b.WriteString("Source file (leave empty for env vars):\n")
	b.WriteString(m.syncFileInput.View())
	b.WriteString("\n\n")

	b.WriteString(RenderKeyHint("tab", "switch field") + "  ")
	b.WriteString(RenderKeyHint("enter", "continue") + "  ")
	b.WriteString(RenderKeyHint("esc", "cancel"))

	return BoxStyle.Width(min(m.width-4, 50)).Render(b.String())
}

func (m Model) renderSyncPreview() string {
	var b strings.Builder

	if len(m.diffs) == 0 {
		b.WriteString(MutedTextStyle.Render("No changes to sync"))
		b.WriteString("\n\n")
		b.WriteString(RenderKeyHint("esc", "back"))
		return b.String()
	}

	// Header
	header := fmt.Sprintf("Sync Preview  %d/%d", m.currentDiffIdx+1, len(m.diffs))
	b.WriteString(TitleStyle.Render(header))
	b.WriteString("\n\n")

	diff := m.diffs[m.currentDiffIdx]

	// Target info
	targetHeader := fmt.Sprintf("%s (%s)", diff.TargetName, diff.Project)
	b.WriteString(BoxStyle.Width(min(m.width-4, 50)).Render(SubtitleStyle.Render(targetHeader)))
	b.WriteString("\n")

	if !diff.HasChanges() {
		b.WriteString(MutedTextStyle.Render("  No changes for this target"))
	} else {
		grouped := diff.GroupByEnvironment()
		for env, diffs := range grouped {
			b.WriteString(fmt.Sprintf("  %s:\n", env))

			changes := 0
			unchanged := 0
			for _, d := range diffs {
				if d.Type == model.DiffUnchanged {
					unchanged++
					continue
				}
				changes++
				b.WriteString(m.renderDiffLine(d))
			}
			if unchanged > 0 {
				b.WriteString(MutedTextStyle.Render(fmt.Sprintf("    = %d unchanged\n", unchanged)))
			}
		}
	}

	b.WriteString("\n")

	// Navigation hints
	hints := []string{}
	if m.currentDiffIdx > 0 {
		hints = append(hints, RenderKeyHint("←", "prev"))
	}
	if m.currentDiffIdx < len(m.diffs)-1 {
		hints = append(hints, RenderKeyHint("→", "next"))
	}

	hasChanges := false
	for _, d := range m.diffs {
		if d.HasChanges() {
			hasChanges = true
			break
		}
	}
	if hasChanges {
		hints = append(hints, RenderKeyHint("enter", "apply all"))
	}
	hints = append(hints, RenderKeyHint("esc", "cancel"))

	b.WriteString(strings.Join(hints, "  "))

	return b.String()
}

func (m Model) renderDiffLine(d model.SecretDiff) string {
	var prefix string
	var style lipgloss.Style

	switch d.Type {
	case model.DiffAdd:
		prefix = "+"
		style = SuccessStyle
	case model.DiffChange:
		prefix = "~"
		style = WarningStyle
	case model.DiffRemove:
		prefix = "-"
		style = ErrorStyle
	case model.DiffUnknown:
		prefix = "?"
		style = InfoStyle
	default:
		return ""
	}

	label := map[model.DiffType]string{
		model.DiffAdd:     "new",
		model.DiffChange:  "changed",
		model.DiffRemove:  "removed",
		model.DiffUnknown: "unknown",
	}[d.Type]

	return fmt.Sprintf("    %s %s %s\n",
		style.Render(prefix),
		d.Name,
		style.Render("("+label+")"))
}

func (m Model) renderSyncing() string {
	var b strings.Builder
	b.WriteString(TitleStyle.Render("Syncing..."))
	b.WriteString("\n\n")
	b.WriteString(m.syncSpinner.View() + " " + m.syncStatus)
	return b.String()
}

func (m Model) renderHelp() string {
	if m.showHelp {
		return m.renderFullHelp()
	}

	hints := []string{
		RenderKeyHint("s", "sync"),
		RenderKeyHint("r", "refresh"),
		RenderKeyHint("tab", "switch"),
		RenderKeyHint("?", "help"),
		RenderKeyHint("q", "quit"),
	}
	return strings.Join(hints, "  ")
}

func (m Model) renderFullHelp() string {
	var b strings.Builder
	b.WriteString(SubtitleStyle.Render("Keyboard Shortcuts"))
	b.WriteString("\n\n")
	b.WriteString("  ↑/k, ↓/j    Navigate items\n")
	b.WriteString("  ←/h, →/l    Navigate targets (sync preview)\n")
	b.WriteString("  Tab         Switch between panels\n")
	b.WriteString("  s           Start sync\n")
	b.WriteString("  r           Refresh config\n")
	b.WriteString("  Enter       Confirm/Apply\n")
	b.WriteString("  Esc         Go back\n")
	b.WriteString("  ?           Toggle help\n")
	b.WriteString("  q           Quit\n")
	return BoxStyle.Render(b.String())
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Run starts the TUI
func Run(configPath string) error {
	p := tea.NewProgram(New(configPath), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
