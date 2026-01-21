package stowry

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
)

type MetaData struct {
	ID            uuid.UUID `json:"id"`
	Path          string    `json:"path"`
	ContentType   string    `json:"content_type"`
	Etag          string    `json:"etag"`
	FileSizeBytes int64     `json:"file_size_bytes"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ObjectEntry struct {
	Path        string
	Size        int64
	ETag        string
	ContentType string
}

type ListQuery struct {
	PathPrefix string
	Limit      int
	Cursor     string
}

type ListResult struct {
	Items      []MetaData `json:"items"`
	NextCursor string     `json:"next_cursor,omitempty"`
}

type SaveResult struct {
	BytesWritten int64
	Etag         string
}

type CreateObject struct {
	Path        string
	ContentType string
}

type ServerMode string

const (
	ModeStore  ServerMode = "store"
	ModeStatic ServerMode = "static"
	ModeSPA    ServerMode = "spa"
)

func (m ServerMode) IsValid() bool {
	switch m {
	case ModeStore, ModeStatic, ModeSPA:
		return true
	default:
		return false
	}
}

func ParseServerMode(s string) (ServerMode, error) {
	mode := ServerMode(s)
	if !mode.IsValid() {
		return "", fmt.Errorf("invalid server mode: %s (valid modes: store, static, spa)", s)
	}
	return mode, nil
}

// Tables holds configurable table names for metadata storage.
// This allows multi-tenant deployments to use different table names.
type Tables struct {
	MetaData string `mapstructure:"meta_data"`
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
