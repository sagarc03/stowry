package e2e_test

import (
	"context"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	pgcontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
)

var (
	testPool     *pgxpool.Pool
	testPoolOnce sync.Once
	testCleanup  func()
	testDSN      string
)

// getSharedPostgresDatabase returns a shared PostgreSQL database for E2E tests.
// The container is reused across all tests for performance.
func getSharedPostgresDatabase(t *testing.T) (dsn string) {
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
		testDSN = connectionStr
	})

	return testDSN
}
