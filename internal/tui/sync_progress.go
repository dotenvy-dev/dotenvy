package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dotenvy-dev/dotenvy/internal/model"
	"github.com/dotenvy-dev/dotenvy/internal/source"
	"github.com/dotenvy-dev/dotenvy/internal/sync"
)

// SyncTask represents a sync operation to perform
type SyncTask struct {
	Target    model.Target
	RemoteEnv string
}

// SyncConfig holds configuration for the sync UI
type SyncConfig struct {
	SecretNames []string
	Source      source.Source
	Tasks       []SyncTask
	DryRun      bool
	LocalEnv    string
}

// SyncUI styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212")).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	successBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("0")).
			Background(lipgloss.Color("42")).
			Padding(0, 1)

	errorBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("0")).
			Background(lipgloss.Color("196")).
			Padding(0, 1)

	warningBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("0")).
			Background(lipgloss.Color("214")).
			Padding(0, 1)

	dryRunBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("0")).
			Background(lipgloss.Color("99")).
			Padding(0, 1)

	targetStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("86"))

	envStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212"))

	addIcon    = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("●")
	changeIcon = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("●")
	skipIcon   = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("○")
	errorIcon  = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("✗")
	checkIcon  = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("✓")

	dimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

// TaskStatus represents the status of a sync task
type TaskStatus int

const (
	TaskPending TaskStatus = iota
	TaskRunning
	TaskComplete
	TaskFailed
)

// TaskResult holds the result of a sync task
type TaskResult struct {
	Task    SyncTask
	Status  TaskStatus
	Diff    *model.TargetDiff
	Result  *sync.SyncResult
	Error   error
	Changes []ChangeItem
}

// ChangeItem represents a single change
type ChangeItem struct {
	Name   string
	Type   model.DiffType
	Status string // "pending", "done", "error"
}

// SyncModel is the bubbletea model for sync UI
type SyncModel struct {
	config      SyncConfig
	engine      *sync.Engine
	ctx         context.Context

	// UI state
	currentTask int
	taskResults []TaskResult
	phase       string // "auth", "preview", "sync", "done"

	// Components
	spinner  spinner.Model
	progress progress.Model

	// Stats
	totalAdded     int
	totalChanged   int
	totalUnknown   int
	totalUnchanged int
	totalFailed    int
	startTime      time.Time
	endTime        time.Time

	// Display
	width  int
	height int
	done   bool
	err    error
}

// Messages
type authCheckMsg struct {
	target string
	ok     bool
	err    error
}

type previewDoneMsg struct {
	taskIndex int
	diff      *model.TargetDiff
	err       error
}

type syncDoneMsg struct {
	taskIndex int
	result    *sync.SyncResult
	err       error
}

type allDoneMsg struct{}

type tickMsg time.Time

// NewSyncModel creates a new sync UI model
func NewSyncModel(cfg SyncConfig) SyncModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
		progress.WithoutPercentage(),
	)

	results := make([]TaskResult, len(cfg.Tasks))
	for i, task := range cfg.Tasks {
		results[i] = TaskResult{
			Task:   task,
			Status: TaskPending,
		}
	}

	return SyncModel{
		config:      cfg,
		engine:      sync.NewEngine(),
		ctx:         context.Background(),
		taskResults: results,
		phase:       "auth",
		spinner:     s,
		progress:    p,
		startTime:   time.Now(),
		width:       80,
	}
}

func (m SyncModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.checkAuth(),
		tickEvery(),
	)
}

func tickEvery() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m SyncModel) checkAuth() tea.Cmd {
	return func() tea.Msg {
		// Check auth for all targets
		for _, task := range m.config.Tasks {
			if task.Target.Type == "dotenv" {
				continue
			}
			status := m.engine.CheckAuth(task.Target)
			if !status.Authenticated {
				return authCheckMsg{target: task.Target.Name, ok: false, err: status.Error}
			}
		}
		return authCheckMsg{ok: true}
	}
}

func (m SyncModel) runPreview(taskIndex int) tea.Cmd {
	return func() tea.Msg {
		task := m.config.Tasks[taskIndex]
		diff, err := m.engine.Preview(
			m.ctx,
			m.config.SecretNames,
			m.config.Source,
			task.Target,
			task.RemoteEnv,
		)
		return previewDoneMsg{taskIndex: taskIndex, diff: diff, err: err}
	}
}

func (m SyncModel) runSync(taskIndex int) tea.Cmd {
	return func() tea.Msg {
		task := m.config.Tasks[taskIndex]
		result, err := m.engine.Sync(
			m.ctx,
			m.config.SecretNames,
			m.config.Source,
			task.Target,
			task.RemoteEnv,
			sync.SyncOptions{DryRun: m.config.DryRun},
		)
		return syncDoneMsg{taskIndex: taskIndex, result: result, err: err}
	}
}

func (m SyncModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.progress.Width = min(msg.Width-20, 50)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case "enter":
			if m.done {
				return m, tea.Quit
			}
		}

	case tickMsg:
		return m, tickEvery()

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case authCheckMsg:
		if !msg.ok {
			m.err = fmt.Errorf("auth failed for %s: %v", msg.target, msg.err)
			m.done = true
			return m, nil
		}
		// Auth passed, start previewing
		m.phase = "preview"
		m.currentTask = 0
		if len(m.config.Tasks) > 0 {
			m.taskResults[0].Status = TaskRunning
			return m, m.runPreview(0)
		}
		m.done = true
		return m, nil

	case previewDoneMsg:
		if msg.err != nil {
			m.taskResults[msg.taskIndex].Status = TaskFailed
			m.taskResults[msg.taskIndex].Error = msg.err
		} else {
			m.taskResults[msg.taskIndex].Diff = msg.diff
			// Build change items
			var changes []ChangeItem
			for _, d := range msg.diff.Diffs {
				if d.Type != model.DiffUnchanged {
					changes = append(changes, ChangeItem{
						Name:   d.Name,
						Type:   d.Type,
						Status: "pending",
					})
				}
			}
			m.taskResults[msg.taskIndex].Changes = changes
		}

		// Move to sync phase for this task
		m.phase = "sync"
		return m, m.runSync(msg.taskIndex)

	case syncDoneMsg:
		if msg.err != nil {
			m.taskResults[msg.taskIndex].Status = TaskFailed
			m.taskResults[msg.taskIndex].Error = msg.err
			m.totalFailed++
		} else {
			m.taskResults[msg.taskIndex].Status = TaskComplete
			m.taskResults[msg.taskIndex].Result = msg.result
			// Mark all changes as done
			for i := range m.taskResults[msg.taskIndex].Changes {
				m.taskResults[msg.taskIndex].Changes[i].Status = "done"
			}
			// Accumulate stats
			if msg.result != nil {
				m.totalAdded += msg.result.Added
				m.totalChanged += msg.result.Changed
				m.totalUnknown += msg.result.Unknown
				m.totalUnchanged += msg.result.Unchanged
				m.totalFailed += msg.result.Failed
			}
		}

		// Move to next task
		m.currentTask++
		if m.currentTask < len(m.config.Tasks) {
			m.taskResults[m.currentTask].Status = TaskRunning
			m.phase = "preview"
			return m, m.runPreview(m.currentTask)
		}

		// All done
		m.phase = "done"
		m.done = true
		m.endTime = time.Now()
		return m, nil
	}

	return m, nil
}

func (m SyncModel) View() string {
	var b strings.Builder

	// Header
	b.WriteString(m.renderHeader())
	b.WriteString("\n")

	// Error state
	if m.err != nil {
		b.WriteString(errorBadge.Render("ERROR"))
		b.WriteString(" ")
		b.WriteString(m.err.Error())
		b.WriteString("\n\n")
		b.WriteString(dimStyle.Render("Press q to exit"))
		return b.String()
	}

	// Progress
	b.WriteString(m.renderProgress())
	b.WriteString("\n")

	// Tasks
	b.WriteString(m.renderTasks())

	// Summary (when done)
	if m.done {
		b.WriteString("\n")
		b.WriteString(m.renderSummary())
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("Press enter to exit"))
	}

	return b.String()
}

func (m SyncModel) renderHeader() string {
	var b strings.Builder

	title := "dotenvy sync"
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n")

	// Source info
	sourceInfo := fmt.Sprintf("Source: %s → %s environment",
		subtitleStyle.Render(m.config.Source.Name()),
		envStyle.Render(m.config.LocalEnv))
	b.WriteString(sourceInfo)

	if m.config.DryRun {
		b.WriteString("  ")
		b.WriteString(dryRunBadge.Render("DRY RUN"))
	}

	return b.String()
}

func (m SyncModel) renderProgress() string {
	var b strings.Builder

	// Calculate progress
	completed := 0
	for _, r := range m.taskResults {
		if r.Status == TaskComplete || r.Status == TaskFailed {
			completed++
		}
	}
	total := len(m.taskResults)
	pct := float64(completed) / float64(total)

	// Progress bar
	b.WriteString("\n")
	b.WriteString(m.progress.ViewAs(pct))
	b.WriteString(" ")
	b.WriteString(dimStyle.Render(fmt.Sprintf("%d/%d targets", completed, total)))

	// Spinner when running
	if !m.done {
		b.WriteString("  ")
		b.WriteString(m.spinner.View())
	}

	return b.String()
}

func (m SyncModel) renderTasks() string {
	var b strings.Builder

	b.WriteString("\n")

	for i, result := range m.taskResults {
		b.WriteString(m.renderTask(i, result))
		b.WriteString("\n")
	}

	return b.String()
}

func (m SyncModel) renderTask(index int, result TaskResult) string {
	var b strings.Builder

	// Status icon
	var statusIcon string
	switch result.Status {
	case TaskPending:
		statusIcon = dimStyle.Render("○")
	case TaskRunning:
		statusIcon = m.spinner.View()
	case TaskComplete:
		statusIcon = checkIcon
	case TaskFailed:
		statusIcon = errorIcon
	}

	// Target info
	targetInfo := fmt.Sprintf("%s → %s/%s",
		targetStyle.Render(result.Task.Target.Name),
		result.Task.Target.GetProject(),
		envStyle.Render(result.Task.RemoteEnv))

	b.WriteString(fmt.Sprintf("%s %s", statusIcon, targetInfo))

	// Show changes for running/complete tasks
	if result.Status == TaskRunning || result.Status == TaskComplete {
		if len(result.Changes) > 0 {
			b.WriteString("\n")
			b.WriteString(m.renderChanges(result.Changes))
		} else if result.Diff != nil && !result.Diff.HasChanges() {
			b.WriteString("\n")
			b.WriteString("    ")
			b.WriteString(dimStyle.Render("No changes"))
		}
	}

	// Show error
	if result.Status == TaskFailed && result.Error != nil {
		b.WriteString("\n")
		b.WriteString("    ")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render(result.Error.Error()))
	}

	return b.String()
}

func (m SyncModel) renderChanges(changes []ChangeItem) string {
	var b strings.Builder

	// Group changes by type
	var adds, mods, unknowns []ChangeItem
	for _, c := range changes {
		switch c.Type {
		case model.DiffAdd:
			adds = append(adds, c)
		case model.DiffChange:
			mods = append(mods, c)
		case model.DiffUnknown:
			unknowns = append(unknowns, c)
		}
	}

	unknownIcon := lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Render("●")

	// Show adds
	if len(adds) > 0 {
		b.WriteString("    ")
		b.WriteString(addIcon)
		b.WriteString(" ")
		names := make([]string, len(adds))
		for i, c := range adds {
			if c.Status == "done" {
				names[i] = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render(c.Name)
			} else {
				names[i] = c.Name
			}
		}
		b.WriteString(strings.Join(names, ", "))
		b.WriteString(dimStyle.Render(" (new)"))
		b.WriteString("\n")
	}

	// Show changes
	if len(mods) > 0 {
		b.WriteString("    ")
		b.WriteString(changeIcon)
		b.WriteString(" ")
		names := make([]string, len(mods))
		for i, c := range mods {
			if c.Status == "done" {
				names[i] = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render(c.Name)
			} else {
				names[i] = c.Name
			}
		}
		b.WriteString(strings.Join(names, ", "))
		b.WriteString(dimStyle.Render(" (changed)"))
		b.WriteString("\n")
	}

	// Show unknowns
	if len(unknowns) > 0 {
		b.WriteString("    ")
		b.WriteString(unknownIcon)
		b.WriteString(" ")
		names := make([]string, len(unknowns))
		for i, c := range unknowns {
			if c.Status == "done" {
				names[i] = lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Render(c.Name)
			} else {
				names[i] = c.Name
			}
		}
		b.WriteString(strings.Join(names, ", "))
		b.WriteString(dimStyle.Render(" (unknown)"))
		b.WriteString("\n")
	}

	return b.String()
}

func (m SyncModel) renderSummary() string {
	duration := m.endTime.Sub(m.startTime).Round(time.Millisecond)

	addedStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))
	changedStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	unknownSummaryStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	failedStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))

	if m.config.DryRun {
		return fmt.Sprintf("%s Would add: %s, change: %s, unknown: %s, skip: %s %s",
			dryRunBadge.Render("DRY RUN"),
			addedStyle.Render(fmt.Sprintf("%d", m.totalAdded)),
			changedStyle.Render(fmt.Sprintf("%d", m.totalChanged)),
			unknownSummaryStyle.Render(fmt.Sprintf("%d", m.totalUnknown)),
			dimStyle.Render(fmt.Sprintf("%d", m.totalUnchanged)),
			dimStyle.Render(fmt.Sprintf("(%s)", duration.String())))
	} else if m.totalFailed == 0 {
		return fmt.Sprintf("%s Added: %s, changed: %s, unknown: %s, unchanged: %s %s",
			successBadge.Render("SUCCESS"),
			addedStyle.Render(fmt.Sprintf("%d", m.totalAdded)),
			changedStyle.Render(fmt.Sprintf("%d", m.totalChanged)),
			unknownSummaryStyle.Render(fmt.Sprintf("%d", m.totalUnknown)),
			dimStyle.Render(fmt.Sprintf("%d", m.totalUnchanged)),
			dimStyle.Render(fmt.Sprintf("(%s)", duration.String())))
	} else {
		return fmt.Sprintf("%s Added: %d, changed: %d, unknown: %d, failed: %s %s",
			warningBadge.Render("PARTIAL"),
			m.totalAdded, m.totalChanged, m.totalUnknown,
			failedStyle.Render(fmt.Sprintf("%d", m.totalFailed)),
			dimStyle.Render(fmt.Sprintf("(%s)", duration.String())))
	}
}

// RunSyncUI runs the sync UI and returns the results
func RunSyncUI(cfg SyncConfig) error {
	m := NewSyncModel(cfg)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	// Check if there was an error in the model
	if fm, ok := finalModel.(SyncModel); ok && fm.err != nil {
		return fm.err
	}

	return nil
}
