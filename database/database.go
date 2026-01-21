package database

import (
	"context"
	"fmt"

	"github.com/sagarc03/stowry"
	"github.com/sagarc03/stowry/database/postgres"
	"github.com/sagarc03/stowry/database/sqlite"
)

// Database provides methods for managing the database connection and schema.
type Database interface {
	// Ping verifies the database connection is alive.
	Ping(ctx context.Context) error

	// Migrate runs database migrations to create required tables.
	// This is a convenience method - in production, users may run migrations separately.
	Migrate(ctx context.Context) error

	// Validate checks that the database schema matches expected structure.
	// Returns an error if required tables or columns are missing or have wrong types.
	Validate(ctx context.Context) error

	// GetRepo returns the MetaDataRepo for database operations.
	GetRepo() stowry.MetaDataRepo

	// Close closes the database connection.
	Close() error
}

// Config holds the configuration for connecting to a metadata backend.
type Config struct {
	// Type specifies the database type: "sqlite" or "postgres"
	Type string `mapstructure:"type"`
	// DSN is the data source name (connection string)
	DSN string `mapstructure:"dsn"`
	// Tables defines the table names for the database
	Tables stowry.Tables `mapstructure:"tables"`
}

// Connect establishes a connection to the configured database backend.
// Tables should be validated before calling Connect.
// Call Migrate() for convenience migrations or Validate() to verify schema.
func Connect(ctx context.Context, cfg Config) (Database, error) {
	switch cfg.Type {
	case "sqlite":
		return sqlite.Connect(ctx, cfg.DSN, cfg.Tables)
	case "postgres":
		return postgres.Connect(ctx, cfg.DSN, cfg.Tables)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", cfg.Type)
	}
}
