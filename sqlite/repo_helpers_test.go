package sqlite_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"math"
	"math/big"
	"testing"

	"github.com/sagarc03/stowry/sqlite"
	"github.com/stretchr/testify/assert"
	_ "modernc.org/sqlite"
)

func getRandomString(t *testing.T) string {
	t.Helper()
	n, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	assert.NoError(t, err, "random string")
	return fmt.Sprintf("test%x", n.Int64())
}

// getTestDatabase creates an in-memory SQLite database for testing
func getTestDatabase(t *testing.T) (*sql.DB, func()) {
	t.Helper()

	// Use in-memory database with shared cache for testing
	db, err := sql.Open("sqlite", ":memory:")
	assert.NoError(t, err, "failed to open sqlite database")

	cleanup := func() {
		if db != nil {
			_ = db.Close()
		}
	}

	return db, cleanup
}

// setupTestRepo creates a repo with a unique table name for test isolation
func setupTestRepo(t *testing.T) (*sqlite.Repo, func()) {
	t.Helper()

	db, dbCleanup := getTestDatabase(t)
	ctx := context.Background()

	// Use a unique table name for each test to avoid conflicts
	tableName := fmt.Sprintf("metadata_%s", getRandomString(t))
	tables := sqlite.Tables{MetaData: tableName}

	// Migrate the table
	err := sqlite.Migrate(ctx, db, tables)
	assert.NoError(t, err, "failed to migrate")

	repo, err := sqlite.NewRepo(db, tables)
	assert.NoError(t, err, "failed to create repo")

	cleanup := func() {
		// Drop the table after the test
		_ = sqlite.DropTables(ctx, db, tables)
		dbCleanup()
	}

	return repo, cleanup
}
