package cmd

import (
	_ "embed"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/sol-strategies/doublezero-version-sync/internal/config"
	"github.com/spf13/cobra"
)

//go:embed version.txt
var versionFile string

var version = strings.TrimSpace(strings.Split(versionFile, "\n")[0])

var (
	configFile   string
	logLevel     string
	loadedConfig *config.Config
)

var rootCmd = &cobra.Command{
	Use:     "doublezero-version-sync",
	Short:   "Version sync manager for DoubleZero",
	Version: version,
	Long: `DoubleZero Version Sync is a version synchronization manager for DoubleZero.
It monitors the installed DoubleZero version and syncs it with the recommended version for the configured cluster.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Resolve config file path (handle tilde expansion)
		resolvedConfigFile := configFile
		if strings.HasPrefix(configFile, "~/") {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				log.Fatal("failed to get user home directory", "error", err)
			}
			resolvedConfigFile = filepath.Join(homeDir, configFile[2:])
		}

		// Load configuration
		var err error
		loadedConfig, err = config.NewFromConfigFile(resolvedConfigFile)
		if err != nil {
			log.Fatal("failed to load configuration", "error", err)
		}

		loadedConfig.Log.ConfigureWithLevelString(logLevel)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Add global flags here
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "~/doublezero-version-sync/config.yaml", "Path to configuration file (default: ~/doublezero-version-sync/config.yaml)")
	rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "l", "", "Log level (debug, info, warn, error, fatal) - overrides config.yaml log.level if specified")

	// Add subcommands here
	rootCmd.AddCommand(runCmd)
}

