package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sagarc03/stowry"
)

type columnInfo struct {
	name       string
	dataType   string
	isNullable bool
}

func validateTableSchema(ctx context.Context, pool *pgxpool.Pool, tableName string, expectedSchema map[string]columnInfo) error {
	if !stowry.IsValidTableName(tableName) {
		return fmt.Errorf("validate table schema: invalid table name: %s", tableName)
	}

	exists, err := tableExists(ctx, pool, tableName)
	if err != nil {
		return fmt.Errorf("validate table schema: %w", err)
	}

	if !exists {
		return fmt.Errorf("validate table schema: table %s does not exist", tableName)
	}

	query := `
		SELECT column_name, data_type, is_nullable
		FROM information_schema.columns
		WHERE table_name = $1
		ORDER BY ordinal_position
	`

	rows, err := pool.Query(ctx, query, tableName)
	if err != nil {
		return fmt.Errorf("validate table schema: query columns: %w", err)
	}
	defer rows.Close()

	actualColumns := make(map[string]columnInfo)
	for rows.Next() {
		var name, dataType, nullable string
		if err := rows.Scan(&name, &dataType, &nullable); err != nil {
			return fmt.Errorf("validate table schema: scan column: %w", err)
		}
		actualColumns[name] = columnInfo{
			name:       name,
			dataType:   strings.ToLower(dataType),
			isNullable: nullable == "YES",
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

func tableExists(ctx context.Context, pool *pgxpool.Pool, tableName string) (bool, error) {
	var exists bool
	query := `
		SELECT EXISTS (
			SELECT 1 
			FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = $1
		)
	`
	err := pool.QueryRow(ctx, query, tableName).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check table exists: %w", err)
	}
	return exists, nil
}

type tableValidation struct {
	tableName      string
	expectedSchema map[string]columnInfo
}

var metaDataTableSchema = map[string]columnInfo{
	"id":              {"id", "uuid", false},
	"path":            {"path", "text", false},
	"content_type":    {"content_type", "text", false},
	"etag":            {"etag", "text", false},
	"file_size_bytes": {"file_size_bytes", "bigint", false},
	"created_at":      {"created_at", "timestamp with time zone", false},
	"updated_at":      {"updated_at", "timestamp with time zone", false},
	"deleted_at":      {"deleted_at", "timestamp with time zone", true},
	"cleaned_up_at":   {"cleaned_up_at", "timestamp with time zone", true},
}

func getTableValidations(tables stowry.Tables) []tableValidation {
	validations := []tableValidation{}

	validations = append(validations, tableValidation{
		tableName:      tables.MetaData,
		expectedSchema: metaDataTableSchema,
	})

	// Future table validations would be added here:
	// validations = append(validations, tableValidation{
	//     tableName:      tables.Users,
	//     expectedSchema: usersTableSchema,
	// })

	return validations
}

func ValidateSchema(ctx context.Context, pool *pgxpool.Pool, tables stowry.Tables) error {
	validations := getTableValidations(tables)

	for _, validation := range validations {
		if err := validateTableSchema(ctx, pool, validation.tableName, validation.expectedSchema); err != nil {
			return fmt.Errorf("validate schema %s: %w", validation.tableName, err)
		}
	}

	return nil
}
