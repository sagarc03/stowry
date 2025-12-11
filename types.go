package stowry

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type MetaData struct {
	ID            uuid.UUID
	Path          string
	ContentType   string
	Etag          string
	FileSizeBytes int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
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
	Items      []MetaData
	NextCursor string
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
