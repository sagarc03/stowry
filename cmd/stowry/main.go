package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/sagarc03/stowry/config"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Version: version,
	Use:     "stowry",
	Short:   "Object storage server with AWS Sig V4 authentication",
	Long: `Stowry is a lightweight object storage server that provides
a REST API backed by local filesystem storage.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		configFiles, _ := cmd.Flags().GetStringSlice("config")

		cfg, err := config.Load(configFiles, cmd.Flags())
		if err != nil {
			return err
		}

		// Store config in context for subcommands
		cmd.SetContext(config.WithContext(cmd.Context(), cfg))

		setupLogging(cfg)
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringSliceP("config", "c", nil, "config file paths (can be specified multiple times, merged left-to-right; default: ./config.yaml)")
	rootCmd.PersistentFlags().String("db-type", "", "database type: sqlite, postgres (default: sqlite, env: STOWRY_DATABASE_TYPE)")
	rootCmd.PersistentFlags().String("db-dsn", "", "database connection string (default: stowry.db, env: STOWRY_DATABASE_DSN)")
	rootCmd.PersistentFlags().String("storage-path", "", "storage directory path (default: ./data, env: STOWRY_STORAGE_PATH)")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
