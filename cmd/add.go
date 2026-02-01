package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/dotenvy-dev/dotenvy/internal/config"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add [NAME...]",
	Short: "Add secret name(s) to the schema",
	Long: `Add one or more secret names to the dotenvy.yaml schema.

This only adds the secret name to track - values are stored in .env files or
environment variables, not in the config file.

Examples:
  # Add a single secret
  dotenvy add API_KEY

  # Add multiple secrets
  dotenvy add API_KEY DATABASE_URL STRIPE_SECRET_KEY

  # Interactive mode (prompts for name)
  dotenvy add
`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runAdd(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(addCmd)
}

func runAdd(names []string) error {
	// Load existing config or create new
	cfg, err := config.Load(cfgFile)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		cfg = config.NewConfig()
	}

	// If no names provided, prompt for them
	if len(names) == 0 {
		var input string
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Secret name(s)").
					Description("Enter one or more names separated by spaces").
					Placeholder("API_KEY DATABASE_URL").
					Value(&input).
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return fmt.Errorf("at least one name is required")
						}
						return nil
					}),
			),
		)
		if err := form.Run(); err != nil {
			return err
		}
		names = strings.Fields(input)
	}

	// Add each secret name
	var added []string
	var skipped []string

	for _, name := range names {
		name = strings.ToUpper(strings.TrimSpace(name))
		if name == "" {
			continue
		}

		if cfg.HasSecret(name) {
			skipped = append(skipped, name)
		} else {
			cfg.AddSecret(name)
			added = append(added, name)
		}
	}

	// Save config if we added anything
	if len(added) > 0 {
		if err := config.Save(cfg, cfgFile); err != nil {
			return err
		}
		fmt.Printf("Added: %s\n", strings.Join(added, ", "))
	}

	if len(skipped) > 0 {
		fmt.Printf("Already in schema: %s\n", strings.Join(skipped, ", "))
	}

	if len(added) == 0 && len(skipped) == 0 {
		fmt.Println("No secrets added.")
	}

	return nil
}
