package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/dotenvy-dev/dotenvy/internal/api"
	"github.com/dotenvy-dev/dotenvy/internal/config"
	"github.com/dotenvy-dev/dotenvy/internal/model"
	"github.com/dotenvy-dev/dotenvy/internal/source"
	"github.com/dotenvy-dev/dotenvy/internal/sync"
	"github.com/dotenvy-dev/dotenvy/internal/tui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	syncEnv     string
	syncEnvFile string
	syncDryRun  bool
	syncTargets []string
	syncPlain   bool
)

var syncCmd = &cobra.Command{
	Use:   "sync [env-or-file]",
	Short: "Sync secrets from source to targets",
	Long: `Sync secrets from an env file (or environment variables) to targets.

Examples:
  # Sync test environment (looks for .env.test)
  dotenvy sync test

  # Sync from a specific file (infers "test" from filename)
  dotenvy sync .env.test

  # Sync to specific targets only
  dotenvy sync test --to vercel,convex

  # Dry run - show what would change
  dotenvy sync test --dry-run

  # Sync from current environment variables (no file)
  dotenvy sync test --no-file

  # Plain output (no animations)
  dotenvy sync test --plain
`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runSync(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

var syncNoFile bool

func init() {
	syncCmd.Flags().StringVarP(&syncEnv, "env", "e", "", "Environment to sync (overrides inference)")
	syncCmd.Flags().StringVarP(&syncEnvFile, "from", "f", "", "Source env file (overrides inference)")
	syncCmd.Flags().BoolVar(&syncNoFile, "no-file", false, "Sync from environment variables instead of file")
	syncCmd.Flags().BoolVar(&syncDryRun, "dry-run", false, "Preview changes without applying")
	syncCmd.Flags().StringSliceVarP(&syncTargets, "to", "t", nil, "Target(s) to sync to (default: all)")
	syncCmd.Flags().BoolVar(&syncPlain, "plain", false, "Plain text output (no TUI)")
	rootCmd.AddCommand(syncCmd)
}

var (
	addStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	changeStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	unknownStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("99"))
	unchangedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	headerStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
)

// resolveEnvAndFile determines the environment and source file from arguments and flags.
// Supports these patterns:
//   - "dotenvy sync test"       -> env=test, file=.env.test (if exists)
//   - "dotenvy sync .env.test"  -> env=test, file=.env.test
//   - "dotenvy sync .env.local" -> env=local, file=.env.local
//   - "dotenvy sync test --no-file" -> env=test, file="" (use env vars)
//   - Flags --env and --from override inference
func resolveEnvAndFile(args []string) (env string, file string, err error) {
	// Start with flag values (they take precedence)
	env = syncEnv
	file = syncEnvFile

	// If --no-file is set, don't look for a file
	useFile := !syncNoFile

	// Parse positional argument if provided
	if len(args) > 0 {
		arg := args[0]

		// Check if it looks like a file path (contains . or /)
		if strings.Contains(arg, ".") || strings.Contains(arg, string(filepath.Separator)) {
			// It's a file path like ".env.test" or "path/to/.env.test"
			if file == "" {
				file = arg
			}
			// Infer env from filename if not set
			if env == "" {
				env = inferEnvFromFilename(arg)
			}
		} else {
			// It's an environment name like "test" or "live"
			if env == "" {
				env = arg
			}
			// Try to find corresponding .env file if not set
			if file == "" && useFile {
				candidateFile := fmt.Sprintf(".env.%s", arg)
				if _, err := os.Stat(candidateFile); err == nil {
					file = candidateFile
				}
			}
		}
	}

	// Validate we have an environment
	if env == "" {
		return "", "", fmt.Errorf("environment required: dotenvy sync <test|live> or dotenvy sync .env.<env>")
	}

	// If we should use a file but don't have one, check if it exists
	if file == "" && useFile {
		candidateFile := fmt.Sprintf(".env.%s", env)
		if _, err := os.Stat(candidateFile); err == nil {
			file = candidateFile
		}
	}

	return env, file, nil
}

// inferEnvFromFilename extracts the environment from a filename like ".env.test" or ".env.live"
func inferEnvFromFilename(filename string) string {
	base := filepath.Base(filename)

	// Handle patterns like ".env.test", ".env.live", ".env.local"
	if strings.HasPrefix(base, ".env.") {
		return strings.TrimPrefix(base, ".env.")
	}

	// Handle patterns like "env.test", "test.env"
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	if ext == ".env" {
		return name
	}
	if strings.HasPrefix(name, "env.") {
		return strings.TrimPrefix(name, "env.")
	}

	// Default: use the extension without the dot
	if ext != "" {
		return strings.TrimPrefix(ext, ".")
	}

	return ""
}

func runSync(args []string) error {
	// Resolve environment and file from arguments
	env, file, err := resolveEnvAndFile(args)
	if err != nil {
		return err
	}

	// Load config
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	secretNames := cfg.GetSecretNames()
	if len(secretNames) == 0 {
		fmt.Println("No secrets defined in config.")
		return nil
	}

	// Build source
	var src source.Source
	if file != "" {
		src = source.NewFileSource(file)
	} else {
		src = source.NewEnvSource()
	}

	// Use resolved env
	syncEnv = env

	// Get targets
	allTargets := cfg.GetTargets()
	var targets []model.Target

	if len(syncTargets) > 0 {
		// Filter to specified targets
		targetSet := make(map[string]bool)
		for _, t := range syncTargets {
			targetSet[t] = true
		}
		for _, t := range allTargets {
			if targetSet[t.Name] {
				targets = append(targets, t)
			}
		}
		if len(targets) == 0 {
			return fmt.Errorf("no matching targets found")
		}
	} else {
		targets = allTargets
	}

	if len(targets) == 0 {
		fmt.Println("No targets configured.")
		return nil
	}

	// Build tasks list
	var tasks []tui.SyncTask
	for _, target := range targets {
		remoteEnvs := target.MapToRemote(syncEnv)
		for _, remoteEnv := range remoteEnvs {
			tasks = append(tasks, tui.SyncTask{
				Target:    target,
				RemoteEnv: remoteEnv,
			})
		}
	}

	if len(tasks) == 0 {
		fmt.Printf("No environment mappings found for '%s'\n", syncEnv)
		return nil
	}

	// Check if we should use TUI or plain output
	// Use plain if: --plain flag, not a TTY, or CI environment
	usePlain := syncPlain || !isTerminal() || os.Getenv("CI") != ""

	// Build API client (nil if no api_key configured)
	apiClient := api.NewClient(cfg.APIKey, cfg.APIURL)

	if usePlain {
		return runSyncPlain(cfg, src, targets, secretNames, apiClient)
	}

	// Run the fancy TUI
	err = tui.RunSyncUI(tui.SyncConfig{
		SecretNames: secretNames,
		Source:      src,
		Tasks:       tasks,
		DryRun:      syncDryRun,
		LocalEnv:    syncEnv,
	})
	if err != nil {
		return err
	}

	// Report events after TUI sync (fire-and-forget)
	if apiClient != nil && !syncDryRun {
		for _, task := range tasks {
			_ = apiClient.ReportEvent(api.Event{
				Action:      "sync",
				Environment: env,
				Target:      task.Target.Name,
				Secrets:     secretNames,
			})
		}
	}

	return nil
}

// isTerminal checks if stdout is a terminal
func isTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// runSyncPlain runs sync with plain text output (no TUI)
func runSyncPlain(cfg *config.Config, src source.Source, targets []model.Target, secretNames []string, apiClient *api.Client) error {
	// Check auth for all targets
	fmt.Println(headerStyle.Render("Checking authentication..."))
	engine := sync.NewEngine()
	allAuth := true
	for _, t := range targets {
		if t.Type == "dotenv" {
			fmt.Printf("  %s %s (local file)\n", successStyle.Render("✓"), t.Name)
			continue
		}
		status := engine.CheckAuth(t)
		if status.Authenticated {
			fmt.Printf("  %s %s\n", successStyle.Render("✓"), t.Name)
		} else {
			fmt.Printf("  %s %s - %v\n", errorStyle.Render("✗"), t.Name, status.Error)
			allAuth = false
		}
	}
	if !allAuth {
		return fmt.Errorf("authentication failed for some targets")
	}

	ctx := context.Background()

	// Sync to each target
	fmt.Println()
	fmt.Println(headerStyle.Render("Syncing secrets..."))
	fmt.Printf("Source: %s\n", src.Name())
	fmt.Printf("Environment: %s\n\n", syncEnv)

	var totalAdded, totalChanged, totalUnknown, totalUnchanged, totalFailed int

	for _, target := range targets {
		// Map local env to remote env(s)
		remoteEnvs := target.MapToRemote(syncEnv)
		if len(remoteEnvs) == 0 {
			fmt.Printf("%s: no mapping for %s environment\n", target.Name, syncEnv)
			continue
		}

		for _, remoteEnv := range remoteEnvs {
			fmt.Printf("%s → %s/%s\n", target.Name, target.GetProject(), remoteEnv)

			// Preview first
			diff, err := engine.Preview(ctx, secretNames, src, target, remoteEnv)
			if err != nil {
				fmt.Printf("  %s %v\n", errorStyle.Render("✗"), err)
				totalFailed++
				continue
			}

			// Show diff
			if !diff.HasChanges() {
				fmt.Printf("  %s\n", unchangedStyle.Render("No changes"))
				totalUnchanged += len(diff.Diffs)
				continue
			}

			for _, d := range diff.Diffs {
				switch d.Type {
				case model.DiffAdd:
					fmt.Printf("  %s %s (new)\n", addStyle.Render("+"), d.Name)
				case model.DiffChange:
					fmt.Printf("  %s %s (changed)\n", changeStyle.Render("~"), d.Name)
				case model.DiffUnknown:
					fmt.Printf("  %s %s (unknown)\n", unknownStyle.Render("?"), d.Name)
				case model.DiffUnchanged:
					// Don't show unchanged
				}
			}

			if syncDryRun {
				counts := diff.CountByType()
				fmt.Printf("  Would add: %d, change: %d, unknown: %d, unchanged: %d\n",
					counts[model.DiffAdd], counts[model.DiffChange], counts[model.DiffUnknown], counts[model.DiffUnchanged])
				continue
			}

			// Apply
			result, err := engine.Sync(ctx, secretNames, src, target, remoteEnv, sync.SyncOptions{})
			if err != nil {
				fmt.Printf("  %s %v\n", errorStyle.Render("✗"), err)
				totalFailed++
				continue
			}

			totalAdded += result.Added
			totalChanged += result.Changed
			totalUnknown += result.Unknown
			totalUnchanged += result.Unchanged
			totalFailed += result.Failed

			if result.Failed > 0 {
				for _, e := range result.Errors {
					fmt.Printf("  %s %v\n", errorStyle.Render("✗"), e)
				}
			}
		}
	}

	// Report events to API (fire-and-forget)
	if apiClient != nil && !syncDryRun {
		for _, target := range targets {
			_ = apiClient.ReportEvent(api.Event{
				Action:      "sync",
				Environment: syncEnv,
				Target:      target.Name,
				Secrets:     secretNames,
			})
		}
	}

	// Summary
	fmt.Println()
	if syncDryRun {
		fmt.Println(unchangedStyle.Render("Dry run - no changes applied"))
	} else {
		if totalFailed == 0 {
			fmt.Printf("%s Added: %d, Changed: %d, Unknown: %d, Unchanged: %d\n",
				successStyle.Render("✓"),
				totalAdded, totalChanged, totalUnknown, totalUnchanged)
		} else {
			fmt.Printf("%s Added: %d, Changed: %d, Unknown: %d, Unchanged: %d, Failed: %d\n",
				errorStyle.Render("!"),
				totalAdded, totalChanged, totalUnknown, totalUnchanged, totalFailed)
		}
	}

	return nil
}
