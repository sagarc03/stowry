package database

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

// cursor is the position of the last row on a page. Rows are ordered by
// (created_at, path), so the pair identifies a row uniquely.
type cursor struct {
	CreatedAt time.Time
	Path      string
}

// encodeCursor renders a position as the opaque token returned to clients.
func encodeCursor(createdAt time.Time, path string) string {
	data := createdAt.Format(time.RFC3339Nano) + "|" + path
	return base64.URLEncoding.EncodeToString([]byte(data))
}

// decodeCursor parses a token from encodeCursor. The empty token decodes to the
// zero cursor, meaning the first row.
func decodeCursor(token string) (cursor, error) {
	if token == "" {
		return cursor{}, nil
	}

	decoded, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		return cursor{}, fmt.Errorf("decode cursor: invalid encoding: %w", err)
	}

	createdAt, path, ok := strings.Cut(string(decoded), "|")
	if !ok {
		return cursor{}, fmt.Errorf("decode cursor: invalid format")
	}

	if path == "" {
		return cursor{}, fmt.Errorf("decode cursor: empty path")
	}

	at, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return cursor{}, fmt.Errorf("decode cursor: invalid timestamp: %w", err)
	}

	return cursor{CreatedAt: at, Path: path}, nil
}

// escapeLikePattern escapes the LIKE wildcards in pattern so that it matches
// literally.
func escapeLikePattern(pattern string) string {
	pattern = strings.ReplaceAll(pattern, `\`, `\\`)
	pattern = strings.ReplaceAll(pattern, `%`, `\%`)
	pattern = strings.ReplaceAll(pattern, `_`, `\_`)
	return pattern
}
