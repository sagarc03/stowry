package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/sagarc03/stowry"
	"github.com/sagarc03/stowry/database"
	"github.com/sagarc03/stowry/filesystem"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize metadata database from storage files",
	Long: `Scan the storage directory and populate the metadata database
with entries for all existing files. This is useful when:
  - Setting up Stowry with existing files
  - Recovering metadata after database loss
  - Migrating from another storage system`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	dbCfg := database.Config{
		Type:  viper.GetString("database.type"),
		DSN:   viper.GetString("database.dsn"),
		Table: viper.GetString("database.table"),
	}

	repo, closeDB, err := database.Connect(ctx, dbCfg)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer closeDB()

	storagePath := viper.GetString("storage.path")
	_, err = os.Stat(storagePath)
	if os.IsNotExist(err) {
		return fmt.Errorf("storage directory does not exist: %s", storagePath)
	}

	root, err := os.OpenRoot(storagePath)
	if err != nil {
		return fmt.Errorf("open storage root: %w", err)
	}
	defer func() { _ = root.Close() }()

	storage := filesystem.NewFileStorage(root)

	service, err := stowry.NewStowryService(repo, storage, stowry.ModeStore)
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}

	slog.Info("scanning storage directory", "path", storagePath)

	if err := service.Populate(ctx); err != nil {
		return fmt.Errorf("populate: %w", err)
	}

	total := 0
	cursor := ""
	for {
		result, err := service.List(ctx, stowry.ListQuery{Limit: 1000, Cursor: cursor})
		if err != nil {
			break
		}
		total += len(result.Items)
		if result.NextCursor == "" {
			break
		}
		cursor = result.NextCursor
	}

	slog.Info("initialization complete", "files_indexed", total)
	return nil
}
