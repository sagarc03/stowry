package postgres_test

// Migration tests validate that table migrations work correctly.
//
// Test Structure:
// - TestMigrate: Validates all tables are created with correct schemas
// - TestDropTables: Validates all tables are properly dropped
// - TestMigrate_DropTables_Integration: Validates round-trip migration
//
// Adding New Tables:
// When you add a new table to migrations.go, update these two functions:
// 1. getExpectedTableSchemas() - Add schema definition with columns/indexes/constraints
// 2. getAllTableNames() - Add table name to the list
//
// The tests will automatically validate the new table's schema.

import (
	"context"
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
	"net"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sagarc03/stowry"
	"github.com/sagarc03/stowry/database/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	pgcontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func getTestDatabase(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := pgcontainer.Run(ctx,
		"postgres:18-alpine",
		pgcontainer.WithDatabase(getRandomString(t)),
		pgcontainer.WithUsername(getRandomString(t)),
		pgcontainer.WithPassword(getRandomString(t)),
		pgcontainer.BasicWaitStrategies(),
		testcontainers.WithExposedPorts(getOpenPort(t)),
	)
	assert.NoError(t, err, "failed to start postgres container")

	cleanup := func() {
		if err := testcontainers.TerminateContainer(pgContainer); err != nil {
			t.Logf("failed to terminate container: %s", err)
		}
	}

	connectionStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		cleanup()
		assert.NoError(t, err, "failed to get connection string")
	}

	pool, err := pgxpool.New(ctx, connectionStr)
	if err != nil {
		cleanup()
		assert.NoError(t, err, "could not connect to database")
	}

	return pool, cleanup
}

func getRandomString(t *testing.T) string {
	t.Helper()
	n, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	assert.NoError(t, err, "random string")
	return fmt.Sprintf("test%x", n.Int64())
}

func getOpenPort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", ":0")
	assert.NoError(t, err, "could not find an open port")

	addr := l.Addr().String()

	err = l.Close()
	assert.NoError(t, err, "could not close port")

	_, port, err := net.SplitHostPort(addr)
	assert.NoError(t, err, "could not parse open port")

	return port
}

type tableSchema struct {
	name                string
	expectedColumns     map[string]string
	expectedIndexes     []string
	hasPrimaryKey       bool
	hasUniqueConstraint bool
}

// getExpectedTableSchemas returns the expected schema for all tables.
// When adding a new table migration:
// 1. Add the table name to stowry.Tables struct
// 2. Add table creation to getTableMigrations() in migrations.go
// 3. Add a new tableSchema entry here with expected columns, indexes, and constraints
// 4. Add the table name to getAllTableNames()
func getExpectedTableSchemas(tables stowry.Tables) []tableSchema {
	return []tableSchema{
		{
			name: tables.MetaData,
			expectedColumns: map[string]string{
				"id":              "uuid",
				"path":            "text",
				"content_type":    "text",
				"etag":            "text",
				"file_size_bytes": "bigint",
				"created_at":      "timestamp with time zone",
				"updated_at":      "timestamp with time zone",
				"deleted_at":      "timestamp with time zone",
				"cleaned_up_at":   "timestamp with time zone",
			},
			expectedIndexes: []string{
				fmt.Sprintf("idx_%s_deleted_at", tables.MetaData),
				fmt.Sprintf("idx_%s_pending_cleanup", tables.MetaData),
			},
			hasPrimaryKey:       true,
			hasUniqueConstraint: true,
		},
		// Add new table schemas here:
		// {
		//     name: tables.Users,
		//     expectedColumns: map[string]string{
		//         "id": "uuid",
		//         "email": "text",
		//         ...
		//     },
		//     expectedIndexes: []string{"idx_users_email"},
		//     hasPrimaryKey: true,
		//     hasUniqueConstraint: true,
		// },
	}
}

// getAllTableNames returns all table names in the order they are created.
// Update this when adding new tables.
func getAllTableNames(tables stowry.Tables) []string {
	return []string{
		tables.MetaData,
		// Add new table names here in creation order:
		// tables.Users,
		// tables.Posts,
	}
}

func verifyTableSchema(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema tableSchema) {
	t.Helper()

	var exists bool
	err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' AND table_name = $1
		)
	`, schema.name).Scan(&exists)
	assert.NoError(t, err, "failed to check table existence for %s", schema.name)
	assert.True(t, exists, "expected table %s to exist", schema.name)

	for colName, expectedType := range schema.expectedColumns {
		var dataType string
		err = pool.QueryRow(ctx, `
			SELECT data_type 
			FROM information_schema.columns 
			WHERE table_name = $1 AND column_name = $2
		`, schema.name, colName).Scan(&dataType)
		assert.NoError(t, err, "table %s: column %s does not exist", schema.name, colName)
		assert.Equal(t, expectedType, dataType, "table %s: column %s type mismatch", schema.name, colName)
	}

	for _, indexName := range schema.expectedIndexes {
		var exists bool
		err = pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT FROM pg_indexes 
				WHERE tablename = $1 AND indexname = $2
			)
		`, schema.name, indexName).Scan(&exists)
		assert.NoError(t, err, "table %s: failed to check index %s", schema.name, indexName)
		assert.True(t, exists, "table %s: expected index %s to exist", schema.name, indexName)
	}

	if schema.hasPrimaryKey {
		var constraintType string
		err = pool.QueryRow(ctx, `
			SELECT constraint_type 
			FROM information_schema.table_constraints 
			WHERE table_name = $1 AND constraint_type = 'PRIMARY KEY'
		`, schema.name).Scan(&constraintType)
		assert.NoError(t, err, "table %s: primary key constraint not found", schema.name)
	}

	if schema.hasUniqueConstraint {
		var hasUnique bool
		err = pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT FROM information_schema.table_constraints 
				WHERE table_name = $1 AND constraint_type = 'UNIQUE'
			)
		`, schema.name).Scan(&hasUnique)
		assert.NoError(t, err, "table %s: failed to check unique constraint", schema.name)
		assert.True(t, hasUnique, "table %s: expected unique constraint", schema.name)
	}
}

func verifyTableDoesNotExist(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tableName string) {
	t.Helper()

	var exists bool
	err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' AND table_name = $1
		)
	`, tableName).Scan(&exists)
	assert.NoError(t, err, "failed to check table existence for %s", tableName)
	assert.False(t, exists, "expected table %s not to exist", tableName)
}

func TestMigrate(t *testing.T) {
	t.Run("success - creates all tables with correct schemas", func(t *testing.T) {
		pool, cleanup := getTestDatabase(t)
		defer cleanup()
		defer pool.Close()

		ctx := context.Background()
		tables := stowry.Tables{MetaData: "metadata"}

		err := postgres.Migrate(ctx, pool, tables)
		assert.NoError(t, err, "Migrate failed")

		schemas := getExpectedTableSchemas(tables)
		for _, schema := range schemas {
			t.Run(schema.name, func(t *testing.T) {
				verifyTableSchema(t, ctx, pool, schema)
			})
		}
	})

	t.Run("idempotent - can run multiple times", func(t *testing.T) {
		pool, cleanup := getTestDatabase(t)
		defer cleanup()
		defer pool.Close()

		ctx := context.Background()
		tables := stowry.Tables{MetaData: "metadata"}

		err := postgres.Migrate(ctx, pool, tables)
		assert.NoError(t, err, "first Migrate failed")

		err = postgres.Migrate(ctx, pool, tables)
		assert.NoError(t, err, "second Migrate failed")
	})
}

func TestDropTables(t *testing.T) {
	t.Run("success - drops all existing tables", func(t *testing.T) {
		pool, cleanup := getTestDatabase(t)
		defer cleanup()
		defer pool.Close()

		ctx := context.Background()
		tables := stowry.Tables{MetaData: "metadata"}

		err := postgres.Migrate(ctx, pool, tables)
		assert.NoError(t, err, "Migrate failed")

		tableNames := getAllTableNames(tables)
		for _, tableName := range tableNames {
			var exists bool
			err = pool.QueryRow(ctx, `
				SELECT EXISTS (
					SELECT FROM information_schema.tables 
					WHERE table_schema = 'public' AND table_name = $1
				)
			`, tableName).Scan(&exists)
			assert.NoError(t, err, "failed to check table existence")
			assert.True(t, exists, "table %s should exist before drop", tableName)
		}

		err = postgres.DropTables(ctx, pool, tables)
		assert.NoError(t, err, "DropTables failed")

		for _, tableName := range tableNames {
			verifyTableDoesNotExist(t, ctx, pool, tableName)
		}
	})

	t.Run("idempotent - can drop multiple times", func(t *testing.T) {
		pool, cleanup := getTestDatabase(t)
		defer cleanup()
		defer pool.Close()

		ctx := context.Background()
		tables := stowry.Tables{MetaData: "metadata"}

		err := postgres.Migrate(ctx, pool, tables)
		assert.NoError(t, err, "Migrate failed")

		err = postgres.DropTables(ctx, pool, tables)
		assert.NoError(t, err, "first DropTables failed")

		err = postgres.DropTables(ctx, pool, tables)
		assert.NoError(t, err, "second DropTables failed")
	})
}

func TestMigrate_DropTables_Integration(t *testing.T) {
	t.Run("round trip - migrate, drop, migrate again", func(t *testing.T) {
		pool, cleanup := getTestDatabase(t)
		defer cleanup()
		defer pool.Close()

		ctx := context.Background()
		tables := stowry.Tables{MetaData: "metadata"}
		tableNames := getAllTableNames(tables)

		err := postgres.Migrate(ctx, pool, tables)
		assert.NoError(t, err, "first Migrate failed")

		for _, tableName := range tableNames {
			var exists bool
			err = pool.QueryRow(ctx, `
				SELECT EXISTS (
					SELECT FROM information_schema.tables 
					WHERE table_schema = 'public' AND table_name = $1
				)
			`, tableName).Scan(&exists)
			assert.NoError(t, err, "failed to check table existence")
			assert.True(t, exists, "table %s should exist after first migrate", tableName)
		}

		err = postgres.DropTables(ctx, pool, tables)
		assert.NoError(t, err, "DropTables failed")

		for _, tableName := range tableNames {
			verifyTableDoesNotExist(t, ctx, pool, tableName)
		}

		err = postgres.Migrate(ctx, pool, tables)
		assert.NoError(t, err, "second Migrate failed")

		for _, tableName := range tableNames {
			var exists bool
			err = pool.QueryRow(ctx, `
				SELECT EXISTS (
					SELECT FROM information_schema.tables 
					WHERE table_schema = 'public' AND table_name = $1
				)
			`, tableName).Scan(&exists)
			assert.NoError(t, err, "failed to check table existence")
			assert.True(t, exists, "table %s should exist after second migrate", tableName)
		}
	})
}
