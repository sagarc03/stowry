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
//	    Type:   "sqlite",
//	    DSN:    "stowry.db",
//	    Tables: stowry.Tables{MetaData: "stowry_metadata"},
//	}
//
//	db, err := database.Connect(ctx, cfg)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer db.Close()
//
//	// Run migrations or validate schema
//	if err := db.Migrate(ctx); err != nil {
//	    log.Fatal(err)
//	}
//
//	repo := db.GetRepo()
//
// # Subpackages
//
// The database package contains backend-specific implementations:
//
//   - database/postgres: PostgreSQL implementation using pgx
//   - database/sqlite: SQLite implementation using modernc.org/sqlite
package database
