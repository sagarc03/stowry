package postgres_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sagarc03/stowry/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	pgcontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
)

var (
	testPool     *pgxpool.Pool
	testPoolOnce sync.Once
	testCleanup  func()
)

// getSharedTestDatabase returns a shared database pool for all tests
// This significantly improves test performance by reusing the same container
func getSharedTestDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()

	testPoolOnce.Do(func() {
		ctx := context.Background()

		pgContainer, err := pgcontainer.Run(ctx,
			"postgres:18-alpine",
			pgcontainer.WithDatabase("testdb"),
			pgcontainer.WithUsername("testuser"),
			pgcontainer.WithPassword("testpass"),
			pgcontainer.BasicWaitStrategies(),
		)
		if err != nil {
			t.Fatalf("failed to start postgres container: %v", err)
		}

		testCleanup = func() {
			if testPool != nil {
				testPool.Close()
			}
			if err := testcontainers.TerminateContainer(pgContainer); err != nil {
				t.Logf("failed to terminate container: %s", err)
			}
		}

		connectionStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
		if err != nil {
			testCleanup()
			t.Fatalf("failed to get connection string: %v", err)
		}

		pool, err := pgxpool.New(ctx, connectionStr)
		if err != nil {
			testCleanup()
			t.Fatalf("could not connect to database: %v", err)
		}

		testPool = pool
	})

	return testPool
}

// setupTestRepo creates a repo with a unique table name for test isolation
func setupTestRepo(t *testing.T) (*postgres.Repo, func()) {
	t.Helper()

	pool := getSharedTestDatabase(t)
	ctx := context.Background()

	// Use a unique table name for each test to avoid conflicts
	tableName := fmt.Sprintf("metadata_%s", getRandomString(t))
	tables := postgres.Tables{MetaData: tableName}

	// Migrate the table
	err := postgres.Migrate(ctx, pool, tables)
	assert.NoError(t, err, "failed to migrate")

	repo, err := postgres.NewRepo(pool, tables)
	assert.NoError(t, err, "failed to create repo")

	cleanup := func() {
		// Drop the table after the test
		_ = postgres.DropTables(ctx, pool, tables)
	}

	return repo, cleanup
}
