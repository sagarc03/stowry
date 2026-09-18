package database_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sagarc03/stowry/internal/database"
	"github.com/sagarc03/stowry/types"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	pgcontainer "github.com/testcontainers/testcontainers-go/modules/postgres"

	_ "modernc.org/sqlite"
)

// fixture is a connected, unmigrated Database on a metadata table of its own.
type fixture struct {
	db    database.Database
	table string
	// exec runs a statement against the same database, for planting schemas
	// that Migrate would never create.
	exec func(t *testing.T, stmt string)
}

// backend is one database the conformance suite runs against.
type backend struct {
	name string
	open func(t *testing.T) fixture
	// badSchemas are tables Validate must reject.
	badSchemas []badSchema
}

// badSchema is a malformed metadata table and the text its rejection must
// mention.
type badSchema struct {
	name        string
	ddl         func(table string) string
	errContains string
}

// migrated returns a fixture's Database with its schema already created.
func (b backend) migrated(t *testing.T) database.Database {
	t.Helper()

	f := b.open(t)
	require.NoError(t, f.db.Migrate(t.Context()))

	return f.db
}

// backends is the set the conformance suite runs against.
func backends() []backend {
	return []backend{sqliteBackend(), postgresBackend()}
}

func sqliteBackend() backend {
	return backend{
		name: "sqlite",
		// A file rather than ":memory:": every connection to ":memory:" gets a
		// private database, so pooled reads and writes would diverge.
		open: func(t *testing.T) fixture {
			t.Helper()

			dsn := filepath.Join(t.TempDir(), "stowry.db")
			table := "metadata"

			db, err := database.Connect(t.Context(), database.Config{
				Type:   "sqlite",
				DSN:    dsn,
				Tables: types.Tables{MetaData: table},
			})
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })

			return fixture{
				db:    db,
				table: table,
				exec: func(t *testing.T, stmt string) {
					t.Helper()

					raw, err := sql.Open("sqlite", dsn)
					require.NoError(t, err)
					defer func() { _ = raw.Close() }()

					_, err = raw.ExecContext(t.Context(), stmt)
					require.NoError(t, err)
				},
			}
		},
		badSchemas: []badSchema{
			{
				name: "missing columns",
				ddl: func(table string) string {
					return fmt.Sprintf(`CREATE TABLE %q (
						id TEXT PRIMARY KEY NOT NULL,
						path TEXT NOT NULL
					)`, table)
				},
				errContains: "missing columns",
			},
			{
				name: "wrong column type",
				ddl: func(table string) string {
					return fmt.Sprintf(`CREATE TABLE %q (
						id TEXT PRIMARY KEY NOT NULL,
						path TEXT NOT NULL,
						content_type TEXT NOT NULL,
						etag TEXT NOT NULL,
						file_size_bytes TEXT NOT NULL,
						created_at TEXT NOT NULL,
						updated_at TEXT NOT NULL,
						deleted_at TEXT,
						cleaned_up_at TEXT
					)`, table)
				},
				errContains: "file_size_bytes",
			},
		},
	}
}

func postgresBackend() backend {
	return backend{
		name: "postgres",
		// One container is shared; isolation comes from the table name.
		open: func(t *testing.T) fixture {
			t.Helper()

			pool := sharedPostgres(t)
			table := "metadata_" + randomSuffix(t)

			db, err := database.Connect(t.Context(), database.Config{
				Type:   "postgres",
				DSN:    pool.Config().ConnString(),
				Tables: types.Tables{MetaData: table},
			})
			require.NoError(t, err)
			t.Cleanup(func() {
				_ = db.Close()
				// context.Background: t.Context is already cancelled here.
				_, _ = pool.Exec(context.Background(),
					fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", pgx.Identifier{table}.Sanitize()))
			})

			return fixture{
				db:    db,
				table: table,
				exec: func(t *testing.T, stmt string) {
					t.Helper()

					_, err := pool.Exec(t.Context(), stmt)
					require.NoError(t, err)
				},
			}
		},
		badSchemas: []badSchema{
			{
				name: "missing columns",
				ddl: func(table string) string {
					return fmt.Sprintf(`CREATE TABLE %s (
						id UUID PRIMARY KEY,
						path TEXT NOT NULL
					)`, pgx.Identifier{table}.Sanitize())
				},
				errContains: "missing columns",
			},
			{
				name: "wrong column type",
				ddl: func(table string) string {
					return fmt.Sprintf(`CREATE TABLE %s (
						id UUID PRIMARY KEY,
						path TEXT NOT NULL,
						content_type TEXT NOT NULL,
						etag TEXT NOT NULL,
						file_size_bytes TEXT NOT NULL,
						created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
						updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
						deleted_at TIMESTAMPTZ,
						cleaned_up_at TIMESTAMPTZ
					)`, pgx.Identifier{table}.Sanitize())
				},
				errContains: "file_size_bytes",
			},
		},
	}
}

var (
	postgresOnce      sync.Once
	postgresPool      *pgxpool.Pool
	postgresContainer testcontainers.Container
	postgresErr       error
)

// sharedPostgres returns a pool for the package's PostgreSQL container,
// starting it on first use. TestMain tears it down.
func sharedPostgres(t *testing.T) *pgxpool.Pool {
	t.Helper()

	postgresOnce.Do(func() {
		ctx := context.Background()

		container, err := pgcontainer.Run(ctx,
			"postgres:18-alpine",
			pgcontainer.WithDatabase("testdb"),
			pgcontainer.WithUsername("testuser"),
			pgcontainer.WithPassword("testpass"),
			pgcontainer.BasicWaitStrategies(),
		)
		if err != nil {
			postgresErr = fmt.Errorf("start postgres container: %w", err)
			return
		}
		postgresContainer = container

		dsn, err := container.ConnectionString(ctx, "sslmode=disable")
		if err != nil {
			postgresErr = fmt.Errorf("postgres connection string: %w", err)
			return
		}

		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			postgresErr = fmt.Errorf("connect to postgres container: %w", err)
			return
		}
		postgresPool = pool
	})

	require.NoError(t, postgresErr)

	return postgresPool
}

func TestMain(m *testing.M) {
	code := m.Run()

	if postgresPool != nil {
		postgresPool.Close()
	}
	if postgresContainer != nil {
		if err := testcontainers.TerminateContainer(postgresContainer); err != nil {
			fmt.Fprintf(os.Stderr, "terminate postgres container: %v\n", err)
		}
	}

	os.Exit(code)
}

// randomSuffix returns a token that keeps concurrent tests off each other's
// tables.
func randomSuffix(t *testing.T) string {
	t.Helper()

	var b [8]byte
	_, err := rand.Read(b[:])
	require.NoError(t, err)

	return hex.EncodeToString(b[:])
}
