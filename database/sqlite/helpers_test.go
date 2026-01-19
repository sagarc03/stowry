package sqlite_test

import (
	"context"
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
	"testing"

	"github.com/sagarc03/stowry"
	"github.com/sagarc03/stowry/database/sqlite"
	"github.com/stretchr/testify/assert"
)

func getRandomString(t *testing.T) string {
	t.Helper()
	n, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	assert.NoError(t, err, "random string")
	return fmt.Sprintf("test%x", n.Int64())
}

// setupTestRepo creates a repo with a unique table name for test isolation
func setupTestRepo(t *testing.T) (stowry.MetaDataRepo, func()) {
	t.Helper()

	ctx := context.Background()

	// Use a unique table name for each test to avoid conflicts
	tableName := fmt.Sprintf("metadata_%s", getRandomString(t))
	tables := stowry.Tables{MetaData: tableName}

	// Connect to in-memory database
	db, err := sqlite.Connect(ctx, ":memory:", tables)
	assert.NoError(t, err, "failed to connect")

	// Migrate the table
	err = db.Migrate(ctx)
	assert.NoError(t, err, "failed to migrate")

	repo := db.GetRepo()

	cleanup := func() {
		_ = db.Close()
	}

	return repo, cleanup
}
