package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sagarc03/stowry"
	"github.com/sagarc03/stowry/database/postgres"
	"github.com/sagarc03/stowry/database/sqlite"

	_ "modernc.org/sqlite" // SQLite driver
)

// Config holds the configuration for connecting to a metadata backend.
type Config struct {
	// Type specifies the database type: "sqlite" or "postgres"
	Type string
	// DSN is the data source name (connection string)
	DSN string
	// Table is the name of the metadata table
	Table string
}

// Connect establishes a connection to the configured database backend,
// runs migrations, validates the schema, and returns a MetaDataRepo.
// The returned cleanup function should be called to close the connection.
func Connect(ctx context.Context, cfg Config) (stowry.MetaDataRepo, func(), error) {
	tables := stowry.Tables{MetaData: cfg.Table}

	switch cfg.Type {
	case "sqlite":
		return connectSQLite(ctx, cfg.DSN, tables)
	case "postgres":
		return connectPostgres(ctx, cfg.DSN, tables)
	default:
		return nil, nil, fmt.Errorf("unsupported database type: %s", cfg.Type)
	}
}

func connectSQLite(ctx context.Context, dsn string, tables stowry.Tables) (stowry.MetaDataRepo, func(), error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("open sqlite: %w", err)
	}

	if err = db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("ping sqlite: %w", err)
	}

	if err = sqlite.Migrate(ctx, db, tables); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("migrate sqlite: %w", err)
	}

	if err = sqlite.ValidateSchema(ctx, db, tables); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("validate sqlite schema: %w", err)
	}

	repo, err := sqlite.NewRepo(db, tables)
	if err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("create sqlite repo: %w", err)
	}

	cleanup := func() {
		_ = db.Close()
	}

	return repo, cleanup, nil
}

func connectPostgres(ctx context.Context, dsn string, tables stowry.Tables) (stowry.MetaDataRepo, func(), error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("connect postgres: %w", err)
	}

	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("ping postgres: %w", err)
	}

	if err = postgres.Migrate(ctx, pool, tables); err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("migrate postgres: %w", err)
	}

	if err = postgres.ValidateSchema(ctx, pool, tables); err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("validate postgres schema: %w", err)
	}

	repo, err := postgres.NewRepo(pool, tables)
	if err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("create postgres repo: %w", err)
	}

	return repo, pool.Close, nil
}
