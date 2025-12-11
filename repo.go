package stowry

import (
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Tables holds configurable table names for metadata storage.
// This allows multi-tenant deployments to use different table names.
type Tables struct {
	MetaData string
}

var validTableNameRegex = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

// IsValidTableName checks if a table name is valid (lowercase, alphanumeric with underscores, max 63 chars).
func IsValidTableName(name string) bool {
	return validTableNameRegex.MatchString(name) && len(name) <= 63
}

// Validate checks that all required table names are set and valid.
func (t Tables) Validate() error {
	if t.MetaData == "" {
		return errors.New("validate tables: metadata table name cannot be empty")
	}

	if !IsValidTableName(t.MetaData) {
		return fmt.Errorf("validate tables: invalid metadata table name: %s (must match ^[a-z_][a-z0-9_]*$ and be <= 63 chars)", t.MetaData)
	}

	return nil
}

// Cursor represents pagination cursor data for list operations.
type Cursor struct {
	CreatedAt time.Time
	Path      string
}

// EncodeCursor encodes cursor data to a base64 string for pagination.
func EncodeCursor(createdAt time.Time, path string) string {
	data := createdAt.Format(time.RFC3339Nano) + "|" + path
	return base64.URLEncoding.EncodeToString([]byte(data))
}

// DecodeCursor decodes a pagination cursor string back to cursor data.
func DecodeCursor(cursor string) (Cursor, error) {
	if cursor == "" {
		return Cursor{}, nil
	}

	decoded, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return Cursor{}, fmt.Errorf("decode cursor: invalid encoding: %w", err)
	}

	parts := strings.SplitN(string(decoded), "|", 2)
	if len(parts) != 2 {
		return Cursor{}, fmt.Errorf("decode cursor: invalid format")
	}

	if parts[1] == "" {
		return Cursor{}, fmt.Errorf("decode cursor: empty path")
	}

	createdAt, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return Cursor{}, fmt.Errorf("decode cursor: invalid timestamp: %w", err)
	}

	return Cursor{CreatedAt: createdAt, Path: parts[1]}, nil
}

// EscapeLikePattern escapes special LIKE characters (%, _, \) to prevent SQL injection.
func EscapeLikePattern(pattern string) string {
	pattern = strings.ReplaceAll(pattern, `\`, `\\`)
	pattern = strings.ReplaceAll(pattern, `%`, `\%`)
	pattern = strings.ReplaceAll(pattern, `_`, `\_`)
	return pattern
}
