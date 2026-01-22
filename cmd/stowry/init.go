package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/sagarc03/stowry"
	"github.com/sagarc03/stowry/config"
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
	cfg, err := config.FromContext(cmd.Context())
	if err != nil {
		return err
	}

	ctx := cmd.Context()

	if err = cfg.Database.Tables.Validate(); err != nil {
		return fmt.Errorf("invalid database config: %w", err)
	}

	db, err := database.Connect(ctx, cfg.Database)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer func() { _ = db.Close() }()

	if err = db.Ping(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	// Always migrate for init command - we're setting up the database
	if err = db.Migrate(ctx); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}

	if err = db.Validate(ctx); err != nil {
		return fmt.Errorf("validate database schema: %w", err)
	}

	repo := db.GetRepo()

	_, err = os.Stat(cfg.Storage.Path)
	if os.IsNotExist(err) {
		return fmt.Errorf("storage directory does not exist: %s", cfg.Storage.Path)
	}

	root, err := os.OpenRoot(cfg.Storage.Path)
	if err != nil {
		return fmt.Errorf("open storage root: %w", err)
	}
	defer func() { _ = root.Close() }()

	storage := filesystem.NewFileStorage(root)

	serviceCfg := stowry.ServiceConfig{Mode: stowry.ModeStore}
	service, err := stowry.NewStowryService(repo, storage, serviceCfg)
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}

	slog.Info("scanning storage directory", "path", cfg.Storage.Path)

	if err := service.Populate(ctx); err != nil {
		return fmt.Errorf("populate: %w", err)
	}

	total := 0
	cursor := ""
	for {
		result, err := service.List(ctx, stowry.ListQuery{Limit: 1000, Cursor: cursor})
		if err != nil {
			return fmt.Errorf("count files: %w", err)
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
