package cmd

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/dotenvy-dev/dotenvy/internal/config"
	"github.com/dotenvy-dev/dotenvy/internal/sync"
	"github.com/dotenvy-dev/dotenvy/pkg/provider"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current configuration status",
	Long:  `Display the current schema, targets, and authentication status.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runStatus(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	mutedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

func runStatus() error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	fmt.Println(titleStyle.Render("dotenvy status"))
	fmt.Println()

	// Secrets schema
	secrets := cfg.GetSecretNames()
	fmt.Printf("Secrets: %d in schema\n", len(secrets))
	for _, name := range secrets {
		fmt.Printf("  %s\n", name)
	}
	fmt.Println()

	// Targets and auth status
	targets := cfg.GetTargets()
	fmt.Printf("Targets: %d configured\n", len(targets))

	engine := sync.NewEngine()
	for _, t := range targets {
		var authStatus string
		if t.Type == "dotenv" {
			path := t.Config["path"]
			if path == nil {
				path = ".env"
			}
			authStatus = mutedStyle.Render(fmt.Sprintf("(file: %s)", path))
		} else {
			status := engine.CheckAuth(t)
			if status.Authenticated {
				src := status.Source
				if status.Source == "env" && status.EnvVar != "" {
					src = status.EnvVar
				}
				authStatus = successStyle.Render(fmt.Sprintf("authenticated (via %s)", src))
			} else {
				hint := ""
				if status.EnvVar != "" {
					hint = fmt.Sprintf(" - set %s", status.EnvVar)
				}
				authStatus = errorStyle.Render("not authenticated" + hint)
			}
		}

		provInfo, _ := provider.Get(t.Type)
		displayName := t.Type
		if provInfo.DisplayName != "" {
			displayName = provInfo.DisplayName
		}

		project := t.GetProject()
		if project != "" {
			project = " (" + project + ")"
		}

		tags := ""
		if provInfo.Beta {
			tags += " " + mutedStyle.Render("(beta)")
		}
		if provider.IsWriteOnly(t.Type) {
			tags += " " + mutedStyle.Render("(write-only)")
		}

		fmt.Printf("  %s [%s]%s: %s%s\n", t.Name, displayName, project, authStatus, tags)

		// Show mapping
		for remote, local := range t.Mapping {
			fmt.Printf("    %s -> %s\n", mutedStyle.Render(remote), local)
		}

		// Show filters
		if len(t.Secrets.Include) > 0 {
			fmt.Printf("    include: %v\n", t.Secrets.Include)
		}
		if len(t.Secrets.Exclude) > 0 {
			fmt.Printf("    exclude: %v\n", t.Secrets.Exclude)
		}
	}
	fmt.Println()

	return nil
}
