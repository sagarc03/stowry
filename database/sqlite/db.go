package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/sagarc03/stowry"
)

type columnInfo struct {
	name       string
	dataType   string
	isNullable bool
}

func validateTableSchema(ctx context.Context, db *sql.DB, tableName string, expectedSchema map[string]columnInfo) error {
	if !stowry.IsValidTableName(tableName) {
		return fmt.Errorf("validate table schema: invalid table name: %s", tableName)
	}

	exists, err := tableExists(ctx, db, tableName)
	if err != nil {
		return fmt.Errorf("validate table schema: %w", err)
	}

	if !exists {
		return fmt.Errorf("validate table schema: table %s does not exist", tableName)
	}

	// SQLite uses PRAGMA table_info to get column information
	query := fmt.Sprintf(`PRAGMA table_info(%s)`, quoteIdentifier(tableName))

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("validate table schema: query columns: %w", err)
	}
	defer func() { _ = rows.Close() }()

	actualColumns := make(map[string]columnInfo)
	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull int
		var dfltValue sql.NullString
		var pk int

		if err := rows.Scan(&cid, &name, &dataType, &notNull, &dfltValue, &pk); err != nil {
			return fmt.Errorf("validate table schema: scan column: %w", err)
		}
		actualColumns[name] = columnInfo{
			name:       name,
			dataType:   strings.ToLower(dataType),
			isNullable: notNull == 0,
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("validate table schema: rows error: %w", err)
	}

	var missingColumns []string
	var mismatchedColumns []string

	for colName, expected := range expectedSchema {
		actual, exists := actualColumns[colName]
		if !exists {
			missingColumns = append(missingColumns, colName)
			continue
		}

		if actual.dataType != expected.dataType {
			mismatchedColumns = append(mismatchedColumns,
				fmt.Sprintf("%s: expected %s, got %s", colName, expected.dataType, actual.dataType))
		}

		if actual.isNullable != expected.isNullable {
			mismatchedColumns = append(mismatchedColumns,
				fmt.Sprintf("%s: expected nullable=%v, got nullable=%v", colName, expected.isNullable, actual.isNullable))
		}
	}

	if len(missingColumns) > 0 || len(mismatchedColumns) > 0 {
		var errMsg strings.Builder
		fmt.Fprintf(&errMsg, "table %s schema validation failed:\n", tableName)

		if len(missingColumns) > 0 {
			fmt.Fprintf(&errMsg, "  missing columns: %s\n", strings.Join(missingColumns, ", "))
		}

		if len(mismatchedColumns) > 0 {
			fmt.Fprintf(&errMsg, "  mismatched columns:\n")
			for _, msg := range mismatchedColumns {
				fmt.Fprintf(&errMsg, "    - %s\n", msg)
			}
		}

		return errors.New(errMsg.String())
	}

	return nil
}

func tableExists(ctx context.Context, db *sql.DB, tableName string) (bool, error) {
	var name string
	query := `SELECT name FROM sqlite_master WHERE type='table' AND name=?`
	err := db.QueryRowContext(ctx, query, tableName).Scan(&name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("check table exists: %w", err)
	}
	return true, nil
}

type tableValidation struct {
	tableName      string
	expectedSchema map[string]columnInfo
}

var metaDataTableSchema = map[string]columnInfo{
	"id":              {"id", "text", false},
	"path":            {"path", "text", false},
	"content_type":    {"content_type", "text", false},
	"etag":            {"etag", "text", false},
	"file_size_bytes": {"file_size_bytes", "integer", false},
	"created_at":      {"created_at", "text", false},
	"updated_at":      {"updated_at", "text", false},
	"deleted_at":      {"deleted_at", "text", true},
	"cleaned_up_at":   {"cleaned_up_at", "text", true},
}

func getTableValidations(tables stowry.Tables) []tableValidation {
	validations := []tableValidation{}

	validations = append(validations, tableValidation{
		tableName:      tables.MetaData,
		expectedSchema: metaDataTableSchema,
	})

	return validations
}

func ValidateSchema(ctx context.Context, db *sql.DB, tables stowry.Tables) error {
	validations := getTableValidations(tables)

	for _, validation := range validations {
		if err := validateTableSchema(ctx, db, validation.tableName, validation.expectedSchema); err != nil {
			return fmt.Errorf("validate schema %s: %w", validation.tableName, err)
		}
	}

	return nil
}
