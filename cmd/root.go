package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/dotenvy-dev/dotenvy/internal/config"
	"github.com/dotenvy-dev/dotenvy/internal/tui"

	// Register providers
	_ "github.com/dotenvy-dev/dotenvy/providers/awssm"
	_ "github.com/dotenvy-dev/dotenvy/providers/awsssm"
	_ "github.com/dotenvy-dev/dotenvy/providers/convex"
	_ "github.com/dotenvy-dev/dotenvy/providers/dotenv"
	_ "github.com/dotenvy-dev/dotenvy/providers/flyio"
	_ "github.com/dotenvy-dev/dotenvy/providers/gcpsm"
	_ "github.com/dotenvy-dev/dotenvy/providers/netlify"
	_ "github.com/dotenvy-dev/dotenvy/providers/railway"
	_ "github.com/dotenvy-dev/dotenvy/providers/render"
	_ "github.com/dotenvy-dev/dotenvy/providers/supabase"
	_ "github.com/dotenvy-dev/dotenvy/providers/vercel"
)

var (
	cfgFile string
	version = "dev"
)

var rootCmd = &cobra.Command{
	Use:   "dotenvy",
	Short: "Sync secrets across deployment platforms",
	Long: `dotenvy - A CLI tool for syncing secrets across deployment platforms.

Manage your secrets in a single YAML config file and sync them to
Vercel, Convex, Railway, and other platforms with visual diffs.

Run without arguments to launch the interactive TUI dashboard.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Launch TUI when no subcommand is provided
		if err := runTUI(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Version = version
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "dotenvy.yaml", "config file path")
}

func runTUI() error {
	// Check if config exists
	if !config.Exists(cfgFile) {
		fmt.Println("No dotenvy.yaml found. Run 'dotenvy init' to create one.")
		fmt.Println("")
		fmt.Println("Commands:")
		fmt.Println("  dotenvy init      Create a new dotenvy.yaml config")
		fmt.Println("  dotenvy add       Add or edit a secret")
		fmt.Println("  dotenvy sync      Sync secrets to targets")
		fmt.Println("  dotenvy status    Show current configuration status")
		return nil
	}

	return tui.Run(cfgFile)
}
