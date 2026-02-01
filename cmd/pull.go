package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/charmbracelet/huh"
	"github.com/dotenvy-dev/dotenvy/internal/api"
	"github.com/dotenvy-dev/dotenvy/internal/config"
	"github.com/dotenvy-dev/dotenvy/internal/source"
	"github.com/dotenvy-dev/dotenvy/internal/sync"
	"github.com/dotenvy-dev/dotenvy/pkg/provider"
	"github.com/spf13/cobra"
)

var (
	pullEnv    string
	pullOutput string
)

var pullCmd = &cobra.Command{
	Use:   "pull [target]",
	Short: "Pull secrets from a target to a local file",
	Long: `Pull secrets from a target (like Vercel) and save to a local .env file.

Examples:
  # Pull from Vercel production to .env.live
  dotenvy pull vercel --env production -o .env.live

  # Pull from Vercel development to .env.test
  dotenvy pull vercel --env development -o .env.test

  # Pull and print to stdout
  dotenvy pull vercel --env production

  # Interactive target selection
  dotenvy pull --env production -o .env.live
`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var target string
		if len(args) > 0 {
			target = args[0]
		}
		if err := runPull(target); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	pullCmd.Flags().StringVarP(&pullEnv, "env", "e", "", "Remote environment to pull from (e.g., production, development)")
	pullCmd.Flags().StringVarP(&pullOutput, "output", "o", "", "Output file (default: print to stdout)")
	pullCmd.MarkFlagRequired("env")
	rootCmd.AddCommand(pullCmd)
}

func runPull(targetName string) error {
	// Load config
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	targets := cfg.GetTargets()
	if len(targets) == 0 {
		return fmt.Errorf("no targets configured")
	}

	// If no target specified, prompt
	if targetName == "" {
		options := make([]huh.Option[string], 0, len(targets))
		for _, t := range targets {
			label := fmt.Sprintf("%s (%s)", t.Name, t.Type)
			if provider.IsWriteOnly(t.Type) {
				label += " (write-only)"
			}
			options = append(options, huh.NewOption(label, t.Name))
		}

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Which target do you want to pull from?").
					Options(options...).
					Value(&targetName),
			),
		)
		if err := form.Run(); err != nil {
			return err
		}
	}

	// Find target
	target, ok := cfg.GetTarget(targetName)
	if !ok {
		return fmt.Errorf("target %q not found", targetName)
	}

	// Block pull from write-only providers
	if provider.IsWriteOnly(target.Type) {
		provInfo, _ := provider.Get(target.Type)
		displayName := target.Type
		if provInfo.DisplayName != "" {
			displayName = provInfo.DisplayName
		}
		return fmt.Errorf(
			"cannot pull from %s: %s is a write-only provider (secret values cannot be read back)\n\n  Use 'dotenvy sync' to push secrets to %s instead",
			targetName, displayName, targetName,
		)
	}

	// Check auth
	engine := sync.NewEngine()
	if target.Type != "dotenv" {
		status := engine.CheckAuth(*target)
		if !status.Authenticated {
			return fmt.Errorf("not authenticated for %s: %v", targetName, status.Error)
		}
	}

	ctx := context.Background()

	// Pull secrets
	fmt.Fprintf(os.Stderr, "Pulling from %s/%s...\n", targetName, pullEnv)

	secrets, err := engine.Pull(ctx, *target, pullEnv)
	if err != nil {
		return err
	}

	if len(secrets) == 0 {
		fmt.Fprintln(os.Stderr, "No secrets found.")
		return nil
	}

	// Filter to only secrets in our schema (if any defined)
	schemaNames := cfg.GetSecretNames()
	if len(schemaNames) > 0 {
		schemaSet := make(map[string]bool)
		for _, name := range schemaNames {
			schemaSet[name] = true
		}

		filtered := make(map[string]string)
		for name, value := range secrets {
			if schemaSet[name] {
				filtered[name] = value
			}
		}

		// Also add any secrets not in schema but found remotely
		// (user might want to add them to schema later)
		notInSchema := 0
		for name := range secrets {
			if !schemaSet[name] {
				notInSchema++
			}
		}
		if notInSchema > 0 {
			fmt.Fprintf(os.Stderr, "Note: %d secrets on remote not in your schema (run with -v to see)\n", notInSchema)
		}

		secrets = filtered
	}

	fmt.Fprintf(os.Stderr, "Found %d secrets\n", len(secrets))

	// Output
	if pullOutput != "" {
		// Write to file
		if err := source.WriteEnvFile(pullOutput, secrets); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Written to %s\n", pullOutput)
	} else {
		// Print to stdout (sorted)
		var names []string
		for name := range secrets {
			names = append(names, name)
		}
		sort.Strings(names)

		for _, name := range names {
			fmt.Printf("%s=%s\n", name, secrets[name])
		}
	}

	// Update schema with any new secret names
	newSecrets := 0
	for name := range secrets {
		if !cfg.HasSecret(name) {
			cfg.AddSecret(name)
			newSecrets++
		}
	}

	if newSecrets > 0 {
		if err := config.Save(cfg, cfgFile); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not update config: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "Added %d new secrets to schema\n", newSecrets)
		}
	}

	// Report pull event to API (fire-and-forget)
	if apiClient := api.NewClient(cfg.APIKey, cfg.APIURL); apiClient != nil {
		secretNames := make([]string, 0, len(secrets))
		for name := range secrets {
			secretNames = append(secretNames, name)
		}
		_ = apiClient.ReportEvent(api.Event{
			Action:      "pull",
			Environment: pullEnv,
			Target:      targetName,
			Secrets:     secretNames,
		})
	}

	return nil
}
