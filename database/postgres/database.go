package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sagarc03/stowry"
)

type database struct {
	pool   *pgxpool.Pool
	tables stowry.Tables
}

// Connect establishes a connection to PostgreSQL.
// Tables should be validated before calling Connect.
func Connect(ctx context.Context, dsn string, tables stowry.Tables) (*database, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}

	return &database{
		pool:   pool,
		tables: tables,
	}, nil
}

// Ping verifies the database connection is alive.
func (d *database) Ping(ctx context.Context) error {
	return d.pool.Ping(ctx)
}

// Migrate runs database migrations to create required tables.
func (d *database) Migrate(ctx context.Context) error {
	if err := createMetaTable(ctx, d.pool, d.tables.MetaData); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

// Validate checks that the database schema matches expected structure.
func (d *database) Validate(ctx context.Context) error {
	validations := getTableValidations(d.tables)

	for _, validation := range validations {
		if err := validateTableSchema(ctx, d.pool, validation.tableName, validation.expectedSchema); err != nil {
			return fmt.Errorf("validate schema %s: %w", validation.tableName, err)
		}
	}

	return nil
}

// GetRepo returns the MetaDataRepo for database operations.
func (d *database) GetRepo() stowry.MetaDataRepo {
	return &repo{pool: d.pool, tableName: d.tables.MetaData}
}

// Close closes the database connection pool.
func (d *database) Close() error {
	d.pool.Close()
	return nil
}
