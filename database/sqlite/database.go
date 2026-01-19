package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/sagarc03/stowry"

	_ "modernc.org/sqlite" // SQLite driver
)

// database provides SQLite database operations.
type database struct {
	db     *sql.DB
	tables stowry.Tables
}

// Connect establishes a connection to SQLite.
// Tables should be validated before calling Connect.
func Connect(ctx context.Context, dsn string, tables stowry.Tables) (*database, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("connect sqlite: %w", err)
	}

	return &database{
		db:     db,
		tables: tables,
	}, nil
}

// Ping verifies the database connection is alive.
func (d *database) Ping(ctx context.Context) error {
	return d.db.PingContext(ctx)
}

// Migrate runs database migrations to create required tables.
func (d *database) Migrate(ctx context.Context) error {
	if err := createMetaTable(ctx, d.db, d.tables.MetaData); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

// Validate checks that the database schema matches expected structure.
func (d *database) Validate(ctx context.Context) error {
	validations := getTableValidations(d.tables)

	for _, validation := range validations {
		if err := validateTableSchema(ctx, d.db, validation.tableName, validation.expectedSchema); err != nil {
			return fmt.Errorf("validate schema %s: %w", validation.tableName, err)
		}
	}

	return nil
}

// GetRepo returns the MetaDataRepo for database operations.
func (d *database) GetRepo() stowry.MetaDataRepo {
	return &repo{db: d.db, tableName: d.tables.MetaData}
}

// Close closes the database connection.
func (d *database) Close() error {
	return d.db.Close()
}
