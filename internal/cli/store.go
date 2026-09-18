package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/afero"

	"github.com/sagarc03/stowry/internal/config"
	"github.com/sagarc03/stowry/internal/database"
)

// openDatabase connects and readies the metadata store. Migrating already ends
// in a validation, so either way the caller gets a schema this binary has
// confirmed it can use.
func openDatabase(ctx context.Context, cfg *config.Config, migrate bool) (database.Database, error) {
	db, err := database.Connect(ctx, cfg.DatabaseConfig())
	if err != nil {
		return nil, fmt.Errorf("connect database: %w", err)
	}

	prepare := db.Validate
	if migrate {
		prepare = db.Migrate
	}

	if err := prepare(ctx); err != nil {
		_ = db.Close()

		if !migrate {
			return nil, fmt.Errorf("validate database schema: %w (run 'stowry migrate' to create it)", err)
		}

		return nil, fmt.Errorf("migrate database: %w", err)
	}

	return db, nil
}

// openStorage returns the object filesystem for path, which is a directory or
// config.MemoryPath.
//
// 0o700 is owner-only. A Kubernetes deployment that needs shared access should
// set fsGroup in securityContext and pre-create the directory with 0o750.
func openStorage(path string) (afero.Fs, error) {
	if path == config.MemoryPath {
		return afero.NewMemMapFs(), nil
	}

	if err := os.MkdirAll(path, 0o700); err != nil {
		return nil, fmt.Errorf("create storage directory: %w", err)
	}

	return afero.NewBasePathFs(afero.NewOsFs(), path), nil
}

// warnEphemeral says so when a command's work cannot outlive it. Neither store
// is an error - serve runs on both by default - but for a command whose whole
// purpose is to leave something behind, silence would be misleading.
func warnEphemeral(cfg *config.Config) {
	if cfg.Database.DSN == config.MemoryPath {
		slog.Warn("database is in memory: everything written here is gone when this process exits",
			"database.dsn", config.MemoryPath)
	}

	if cfg.Storage.Path == config.MemoryPath {
		slog.Warn("storage is in memory: it holds no files to record",
			"storage.path", config.MemoryPath)
	}
}
