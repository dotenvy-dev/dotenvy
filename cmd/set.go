package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/dotenvy-dev/dotenvy/internal/api"
	"github.com/dotenvy-dev/dotenvy/internal/config"
	"github.com/dotenvy-dev/dotenvy/internal/source"
	"github.com/dotenvy-dev/dotenvy/internal/tui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	setEnv    string
	setDryRun bool
	setPlain  bool
)

var setCmd = &cobra.Command{
	Use:   "set NAME=VALUE [NAME=VALUE...]",
	Short: "Add secrets and sync to all targets",
	Long: `Add one or more secrets and immediately sync to all targets.

This command:
1. Adds the secret name to dotenvy.yaml (if not already tracked)
2. Writes the value to your .env file
3. Syncs to all configured targets

Examples:
  # Set a secret for test environment (default)
  dotenvy set POSTHOG_KEY=phc_xxx

  # Set multiple secrets at once
  dotenvy set POSTHOG_KEY=phc_xxx POSTHOG_HOST=https://app.posthog.com

  # Set for live/production environment
  dotenvy set STRIPE_KEY=sk_live_xxx --env live

  # Preview without applying
  dotenvy set POSTHOG_KEY=phc_xxx --dry-run
`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runSet(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	setCmd.Flags().StringVarP(&setEnv, "env", "e", "test", "Environment (test or live)")
	setCmd.Flags().BoolVar(&setDryRun, "dry-run", false, "Preview changes without applying")
	setCmd.Flags().BoolVar(&setPlain, "plain", false, "Plain text output (no TUI)")
	rootCmd.AddCommand(setCmd)
}

func runSet(args []string) error {
	// Parse NAME=VALUE pairs
	secrets := make(map[string]string)
	for _, arg := range args {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid format %q: expected NAME=VALUE", arg)
		}
		name := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if name == "" {
			return fmt.Errorf("invalid format %q: name cannot be empty", arg)
		}
		secrets[name] = value
	}

	// Load config
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Add secret names to config if not already tracked
	var added []string
	for name := range secrets {
		if !cfg.HasSecret(name) {
			cfg.AddSecret(name)
			added = append(added, name)
		}
	}

	// Save config if we added new secrets
	if len(added) > 0 {
		if err := config.Save(cfg, cfgFile); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
		fmt.Printf("Added to config: %s\n", strings.Join(added, ", "))
	}

	// Write to .env file
	envFile := fmt.Sprintf(".env.%s", setEnv)
	if err := appendToEnvFile(envFile, secrets); err != nil {
		return fmt.Errorf("failed to write to %s: %w", envFile, err)
	}
	fmt.Printf("Updated %s\n\n", envFile)

	// Now sync
	secretNames := cfg.GetSecretNames()
	src := source.NewFileSource(envFile)

	targets := cfg.GetTargets()
	if len(targets) == 0 {
		fmt.Println("No targets configured. Secrets saved locally.")
		return nil
	}

	// Build API client (nil if no api_key configured)
	apiClient := api.NewClient(cfg.APIKey, cfg.APIURL)

	// Build tasks
	var tasks []tui.SyncTask
	for _, target := range targets {
		remoteEnvs := target.MapToRemote(setEnv)
		for _, remoteEnv := range remoteEnvs {
			tasks = append(tasks, tui.SyncTask{
				Target:    target,
				RemoteEnv: remoteEnv,
			})
		}
	}

	if len(tasks) == 0 {
		fmt.Printf("No environment mappings found for '%s'. Secrets saved locally.\n", setEnv)
		return nil
	}

	// Collect secret names that were set
	setSecretNames := make([]string, 0, len(secrets))
	for name := range secrets {
		setSecretNames = append(setSecretNames, name)
	}

	// Use plain output if not a TTY
	usePlain := setPlain || !term.IsTerminal(int(os.Stdout.Fd())) || os.Getenv("CI") != ""

	if usePlain {
		// Reuse the sync plain logic
		syncEnv = setEnv
		syncDryRun = setDryRun
		return runSyncPlain(cfg, src, targets, secretNames, apiClient)
	}

	err = tui.RunSyncUI(tui.SyncConfig{
		SecretNames: secretNames,
		Source:      src,
		Tasks:       tasks,
		DryRun:      setDryRun,
		LocalEnv:    setEnv,
	})
	if err != nil {
		return err
	}

	// Report set events (fire-and-forget)
	if apiClient != nil && !setDryRun {
		for _, target := range targets {
			_ = apiClient.ReportEvent(api.Event{
				Action:      "set",
				Environment: setEnv,
				Target:      target.Name,
				Secrets:     setSecretNames,
			})
		}
	}

	return nil
}

// appendToEnvFile appends or updates secrets in an env file
func appendToEnvFile(path string, secrets map[string]string) error {
	// Read existing content
	existing := make(map[string]bool)
	var lines []string

	content, err := os.ReadFile(path)
	if err == nil {
		// File exists, parse it
		for _, line := range strings.Split(string(content), "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				lines = append(lines, line)
				continue
			}

			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 {
				name := strings.TrimSpace(parts[0])
				if newVal, ok := secrets[name]; ok {
					// Update existing value
					lines = append(lines, fmt.Sprintf("%s=%s", name, newVal))
					existing[name] = true
					continue
				}
			}
			lines = append(lines, line)
		}
	}

	// Append new secrets that weren't already in the file
	for name, value := range secrets {
		if !existing[name] {
			lines = append(lines, fmt.Sprintf("%s=%s", name, value))
		}
	}

	// Ensure file ends with newline
	output := strings.Join(lines, "\n")
	if !strings.HasSuffix(output, "\n") {
		output += "\n"
	}

	return os.WriteFile(path, []byte(output), 0644)
}
