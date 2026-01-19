package postgres_test

import (
	"context"
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
	"net"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sagarc03/stowry"
	"github.com/sagarc03/stowry/database/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	pgcontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
)

var (
	testPool     *pgxpool.Pool
	testPoolOnce sync.Once
	testCleanup  func()
)

// getSharedTestDatabase returns a shared database pool for all tests.
// This significantly improves test performance by reusing the same container.
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

// getIsolatedTestDatabase returns an isolated database pool for tests that need
// a clean database state. Each call creates a new container.
func getIsolatedTestDatabase(t *testing.T) (*pgxpool.Pool, func()) {
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

// getRandomString generates a random string for unique test identifiers.
func getRandomString(t *testing.T) string {
	t.Helper()
	n, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	assert.NoError(t, err, "random string")
	return fmt.Sprintf("test%x", n.Int64())
}

// getOpenPort finds an available port for test containers.
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

// dropTable drops the specified table for test cleanup.
func dropTable(ctx context.Context, pool *pgxpool.Pool, tableName string) error {
	quotedTable := pgx.Identifier{tableName}.Sanitize()
	sql := fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", quotedTable)
	_, err := pool.Exec(ctx, sql)
	return err
}

// getDSN extracts the DSN from the pool config.
func getDSN(pool *pgxpool.Pool) string {
	return pool.Config().ConnString()
}

// setupTestRepo creates a repo with a unique table name for test isolation.
func setupTestRepo(t *testing.T) (stowry.MetaDataRepo, func()) {
	t.Helper()

	pool := getSharedTestDatabase(t)
	ctx := context.Background()

	// Use a unique table name for each test to avoid conflicts
	tableName := fmt.Sprintf("metadata_%s", getRandomString(t))
	tables := stowry.Tables{MetaData: tableName}

	db, err := postgres.Connect(ctx, getDSN(pool), tables)
	assert.NoError(t, err, "failed to connect")

	// Migrate the table
	err = db.Migrate(ctx)
	assert.NoError(t, err, "failed to migrate")

	cleanup := func() {
		_ = db.Close()
		// Drop the table after the test
		_ = dropTable(ctx, pool, tableName)
	}

	return db.GetRepo(), cleanup
}
