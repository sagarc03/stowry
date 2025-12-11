package main

import (
	"errors"
	"log/slog"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	setDefaults()
}

func setDefaults() {
	viper.SetDefault("server.port", 5708)
	viper.SetDefault("server.mode", "store")

	viper.SetDefault("database.type", "sqlite")
	viper.SetDefault("database.dsn", "stowry.db")
	viper.SetDefault("database.table", "stowry_metadata")

	viper.SetDefault("storage.path", "./data")

	viper.SetDefault("access.public_read", false)
	viper.SetDefault("access.public_write", false)

	viper.SetDefault("auth.region", "us-east-1")
	viper.SetDefault("auth.service", "s3")

	viper.SetDefault("log.level", "info")
}

func readConfig(cmd *cobra.Command) {
	if err := viper.BindPFlags(cmd.Flags()); err != nil {
		slog.Warn("failed to bind flags", "err", err)
	}

	configFile, _ := cmd.Flags().GetString("config")
	if configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
	}

	viper.SetEnvPrefix("STOWRY")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		var configNotFound viper.ConfigFileNotFoundError
		if !errors.As(err, &configNotFound) {
			slog.Warn("error reading config file", "err", err)
		}
	}
}
