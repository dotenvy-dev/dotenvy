package cmd

import (
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/dotenvy-dev/dotenvy/internal/config"
	"github.com/dotenvy-dev/dotenvy/internal/detect"
	"github.com/dotenvy-dev/dotenvy/internal/source"
	"github.com/dotenvy-dev/dotenvy/internal/tui"
	"github.com/spf13/cobra"
)

var initFrom string

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a new dotenvy.yaml configuration",
	Long:  `Initialize a new dotenvy.yaml configuration file with guided setup.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runInit(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	initCmd.Flags().StringVar(&initFrom, "from", "", "Path to .env file to detect providers and import keys from")
	rootCmd.AddCommand(initCmd)
}

func runInit() error {
	// Check if config already exists
	if config.Exists(cfgFile) {
		var overwrite bool
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title(fmt.Sprintf("%s already exists. Overwrite?", cfgFile)).
					Value(&overwrite),
			),
		)
		if err := form.Run(); err != nil {
			return err
		}
		if !overwrite {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	// Detect providers from .env file
	var detectionResult *detect.Result
	envFile := initFrom
	if envFile == "" {
		envFile = detect.FindEnvFile()
	}
	if envFile != "" {
		r, err := detect.FromFile(envFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not read %s: %v\n", envFile, err)
		} else {
			detectionResult = r
			fmt.Println()
			fmt.Print(tui.RenderDetectionSummary(r))
			fmt.Println()
		}
	}

	// Build provider options with pre-selection based on detection
	detected := make(map[string]bool)
	if detectionResult != nil {
		for _, p := range detectionResult.Providers {
			detected[p] = true
		}
	}

	var providers []string
	options := []huh.Option[string]{
		huh.NewOption("Vercel", "vercel").Selected(detected["vercel"]),
		huh.NewOption("Convex", "convex").Selected(detected["convex"]),
		huh.NewOption("Railway", "railway").Selected(detected["railway"]),
		huh.NewOption("Render", "render").Selected(detected["render"]),
		huh.NewOption("Supabase", "supabase").Selected(detected["supabase"]),
		huh.NewOption("Netlify", "netlify").Selected(detected["netlify"]),
		huh.NewOption("Fly.io", "flyio").Selected(detected["flyio"]),
		huh.NewOption("Local .env", "dotenv").Selected(detected["dotenv"]),
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Which providers do you want to sync to?").
				Options(options...).
				Value(&providers),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}

	cfg := config.NewConfig()

	// Configure each selected provider
	for _, p := range providers {
		switch p {
		case "vercel":
			if err := configureVercel(cfg); err != nil {
				return err
			}
		case "convex":
			if err := configureConvex(cfg); err != nil {
				return err
			}
		case "railway":
			if err := configureRailway(cfg); err != nil {
				return err
			}
		case "render":
			if err := configureRender(cfg); err != nil {
				return err
			}
		case "supabase":
			if err := configureSupabase(cfg); err != nil {
				return err
			}
		case "netlify":
			if err := configureNetlify(cfg); err != nil {
				return err
			}
		case "flyio":
			if err := configureFlyio(cfg); err != nil {
				return err
			}
		case "dotenv":
			configureDotenv(cfg)
		}
	}

	// Import all keys from the detected .env file into the secrets list
	if detectionResult != nil {
		for _, key := range detectionResult.AllKeys {
			cfg.AddSecret(key)
		}
	}

	// Bootstrap .env.test with values from the source file
	bootstrapped := false
	if detectionResult != nil && detectionResult.SourceFile != "" {
		fs := source.NewFileSource(detectionResult.SourceFile)
		allSecrets, err := fs.ListAll()
		if err == nil && len(allSecrets) > 0 {
			targetFile := ".env.test"
			// Don't overwrite if source IS .env.test (already in place)
			if detectionResult.SourceFile != targetFile {
				if err := source.WriteEnvFile(targetFile, allSecrets); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: could not write %s: %v\n", targetFile, err)
				} else {
					bootstrapped = true
				}
			}
		}
	}

	// Save config
	if err := config.Save(cfg, cfgFile); err != nil {
		return err
	}

	fmt.Printf("\nCreated %s\n", cfgFile)
	if detectionResult != nil && len(detectionResult.AllKeys) > 0 {
		fmt.Printf("Imported %d secrets from %s\n", len(detectionResult.AllKeys), detectionResult.SourceFile)
	}
	if bootstrapped {
		fmt.Printf("Bootstrapped .env.test with values from %s\n", detectionResult.SourceFile)
	}
	fmt.Println("\nNext steps:")
	if bootstrapped {
		fmt.Println("  1. Run 'dotenvy status' to check authentication")
		fmt.Println("  2. Run 'dotenvy sync test' to sync to all targets")
	} else if detectionResult == nil || detectionResult.SourceFile == "" {
		fmt.Println("  1. Create .env.test and .env.live files with your secrets")
		fmt.Println("  2. Set up authentication (env vars or config)")
		fmt.Println("  3. Run 'dotenvy sync --env test --from .env.test --dry-run' to preview")
		fmt.Println("  4. Run 'dotenvy sync --env test --from .env.test' to apply")
	} else {
		fmt.Printf("  1. Review your secrets in %s\n", cfgFile)
		fmt.Println("  2. Set up authentication (env vars or config)")
		fmt.Println("  3. Run 'dotenvy sync --env test --from .env.test --dry-run' to preview")
		fmt.Println("  4. Run 'dotenvy sync --env test --from .env.test' to apply")
	}

	return nil
}

func configureVercel(cfg *config.Config) error {
	var project string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Vercel project name").
				Placeholder("my-app").
				Value(&project),
		),
	)
	if err := form.Run(); err != nil {
		return err
	}

	cfg.AddTarget("vercel", &config.TargetDef{
		Type:    "vercel",
		Project: project,
		Mapping: map[string]string{
			"development": "test",
			"preview":     "test",
			"production":  "live",
		},
	})

	fmt.Println("\nVercel authentication:")
	fmt.Println("  Set VERCEL_TOKEN environment variable, or add 'token:' to config")
	fmt.Println("  Get token from: https://vercel.com/account/tokens")

	return nil
}

func configureConvex(cfg *config.Config) error {
	var devDeployment, prodDeployment string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Convex dev deployment name").
				Placeholder("my-app-dev").
				Value(&devDeployment),
			huh.NewInput().
				Title("Convex prod deployment name (leave empty to skip)").
				Placeholder("my-app-prod").
				Value(&prodDeployment),
		),
	)
	if err := form.Run(); err != nil {
		return err
	}

	if devDeployment != "" {
		cfg.AddTarget("convex-dev", &config.TargetDef{
			Type:       "convex",
			Deployment: devDeployment,
			Mapping: map[string]string{
				"default": "test",
			},
		})
	}

	if prodDeployment != "" {
		cfg.AddTarget("convex-prod", &config.TargetDef{
			Type:       "convex",
			Deployment: prodDeployment,
			Mapping: map[string]string{
				"default": "live",
			},
		})
	}

	fmt.Println("\nConvex authentication:")
	fmt.Println("  Set CONVEX_DEPLOY_KEY environment variable, or add 'deploy_key:' to config")
	fmt.Println("  Get deploy key from: Convex dashboard → Settings → Deploy Keys")

	return nil
}

func configureRailway(cfg *config.Config) error {
	var projectID, serviceID string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Railway project ID").
				Placeholder("8df3b1d6-2317-4400-b267-56c4a42eed06").
				Value(&projectID),
			huh.NewInput().
				Title("Railway service ID (leave empty for project-level variables)").
				Placeholder("4bd252dc-c4ac-4c2e-a52f-051804292035").
				Value(&serviceID),
		),
	)
	if err := form.Run(); err != nil {
		return err
	}

	def := &config.TargetDef{
		Type:      "railway",
		ProjectID: projectID,
		Mapping: map[string]string{
			"production": "live",
			"staging":    "test",
		},
	}
	if serviceID != "" {
		def.ServiceID = serviceID
	}

	cfg.AddTarget("railway", def)

	fmt.Println("\nRailway authentication:")
	fmt.Println("  Set RAILWAY_TOKEN environment variable, or add 'token:' to config")
	fmt.Println("  Get token from: https://railway.com/account/tokens")

	return nil
}

func configureRender(cfg *config.Config) error {
	var serviceID string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Render service ID").
				Placeholder("srv-abc123def456").
				Value(&serviceID),
		),
	)
	if err := form.Run(); err != nil {
		return err
	}

	cfg.AddTarget("render", &config.TargetDef{
		Type:      "render",
		ServiceID: serviceID,
		Mapping: map[string]string{
			"default": "test",
		},
	})

	fmt.Println("\nRender authentication:")
	fmt.Println("  Set RENDER_API_KEY environment variable, or add 'token:' to config")
	fmt.Println("  Get API key from: https://dashboard.render.com/u/settings#api-keys")

	return nil
}

func configureSupabase(cfg *config.Config) error {
	var projectRef string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Supabase project ref").
				Placeholder("abcdefghijklmnop").
				Value(&projectRef),
		),
	)
	if err := form.Run(); err != nil {
		return err
	}

	cfg.AddTarget("supabase", &config.TargetDef{
		Type:       "supabase",
		ProjectRef: projectRef,
		Mapping: map[string]string{
			"default": "test",
		},
	})

	fmt.Println("\nSupabase authentication:")
	fmt.Println("  Set SUPABASE_ACCESS_TOKEN environment variable, or add 'token:' to config")
	fmt.Println("  Get token from: https://supabase.com/dashboard/account/tokens")
	fmt.Println("  Note: Supabase is write-only - secret values cannot be read back")

	return nil
}

func configureNetlify(cfg *config.Config) error {
	var accountID, siteID string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Netlify account/team ID").
				Placeholder("my-team-slug").
				Value(&accountID),
			huh.NewInput().
				Title("Netlify site ID (leave empty for account-level)").
				Placeholder("abc123-def456").
				Value(&siteID),
		),
	)
	if err := form.Run(); err != nil {
		return err
	}

	def := &config.TargetDef{
		Type:      "netlify",
		AccountID: accountID,
		Mapping: map[string]string{
			"production":     "live",
			"deploy-preview": "test",
			"branch-deploy":  "test",
			"dev":            "test",
		},
	}
	if siteID != "" {
		def.SiteID = siteID
	}

	cfg.AddTarget("netlify", def)

	fmt.Println("\nNetlify authentication:")
	fmt.Println("  Set NETLIFY_TOKEN environment variable, or add 'token:' to config")
	fmt.Println("  Get token from: https://app.netlify.com/user/applications#personal-access-tokens")

	return nil
}

func configureFlyio(cfg *config.Config) error {
	var appName string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Fly.io app name").
				Placeholder("my-app-staging").
				Value(&appName),
		),
	)
	if err := form.Run(); err != nil {
		return err
	}

	cfg.AddTarget("flyio", &config.TargetDef{
		Type:    "flyio",
		AppName: appName,
		Mapping: map[string]string{
			"default": "test",
		},
	})

	fmt.Println("\nFly.io authentication:")
	fmt.Println("  Set FLY_API_TOKEN environment variable, or add 'token:' to config")
	fmt.Println("  Get token from: https://fly.io/user/personal_access_tokens")
	fmt.Println("  Note: Fly.io is write-only - secret values cannot be read back")

	return nil
}

func configureDotenv(cfg *config.Config) {
	cfg.AddTarget("local", &config.TargetDef{
		Type: "dotenv",
		Path: ".env",
		Mapping: map[string]string{
			"local": "test",
		},
	})
}
