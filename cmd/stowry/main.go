package main

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Version: version,
	Use:     "stowry",
	Short:   "Object storage server with AWS Sig V4 authentication",
	Long: `Stowry is a lightweight object storage server that provides
a REST API backed by local filesystem storage.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		readConfig(cmd)
		setupLogging()
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().String("config", "", "config file path (default: ./config.yaml)")
	rootCmd.PersistentFlags().String("db-type", "", "database type: sqlite, postgres (default: sqlite, env: STOWRY_DATABASE_TYPE)")
	rootCmd.PersistentFlags().String("db-dsn", "", "database connection string (default: stowry.db, env: STOWRY_DATABASE_DSN)")
	rootCmd.PersistentFlags().String("storage-path", "", "storage directory path (default: ./data, env: STOWRY_STORAGE_PATH)")

	_ = viper.BindPFlag("database.type", rootCmd.PersistentFlags().Lookup("db-type"))
	_ = viper.BindPFlag("database.dsn", rootCmd.PersistentFlags().Lookup("db-dsn"))
	_ = viper.BindPFlag("storage.path", rootCmd.PersistentFlags().Lookup("storage-path"))
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
