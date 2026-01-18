// Package database provides a unified interface for connecting to metadata backends.
//
// The package supports multiple database backends (PostgreSQL and SQLite) and handles
// connection management, migrations, and schema validation automatically.
//
// # Supported Backends
//
//   - PostgreSQL: Production-ready backend using pgx connection pool
//   - SQLite: Lightweight backend suitable for development and single-node deployments
//
// # Usage
//
//	cfg := database.Config{
//	    Type:  "sqlite",
//	    DSN:   "stowry.db",
//	    Table: "stowry_metadata",
//	}
//
//	repo, cleanup, err := database.Connect(ctx, cfg)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer cleanup()
//
// The Connect function automatically:
//   - Opens the database connection
//   - Runs schema migrations
//   - Validates the schema
//   - Returns a ready-to-use MetaDataRepo
//
// # Subpackages
//
// The database package contains backend-specific implementations:
//
//   - database/postgres: PostgreSQL implementation using pgx
//   - database/sqlite: SQLite implementation using modernc.org/sqlite
package database
