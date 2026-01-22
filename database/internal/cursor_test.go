package internal_test

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/sagarc03/stowry/database/internal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeCursor_DecodeCursor_RoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		createdAt time.Time
		path      string
	}{
		{
			name:      "simple path",
			createdAt: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			path:      "test/file.txt",
		},
		{
			name:      "path with special characters",
			createdAt: time.Date(2024, 6, 20, 14, 45, 30, 123456789, time.UTC),
			path:      "folder/sub-folder/file_name.json",
		},
		{
			name:      "nanosecond precision",
			createdAt: time.Date(2024, 12, 31, 23, 59, 59, 999999999, time.UTC),
			path:      "precision-test.bin",
		},
		{
			name:      "path with pipe character",
			createdAt: time.Date(2024, 3, 10, 8, 0, 0, 0, time.UTC),
			path:      "path|with|pipes.txt",
		},
		{
			name:      "deeply nested path",
			createdAt: time.Date(2024, 5, 5, 12, 0, 0, 0, time.UTC),
			path:      "a/b/c/d/e/f/g/h/i/j/file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			encoded := internal.EncodeCursor(tt.createdAt, tt.path)
			assert.NotEmpty(t, encoded, "encoded cursor should not be empty")

			decoded, err := internal.DecodeCursor(encoded)
			require.NoError(t, err)

			assert.True(t, tt.createdAt.Equal(decoded.CreatedAt),
				"createdAt mismatch: expected %v, got %v", tt.createdAt, decoded.CreatedAt)
			assert.Equal(t, tt.path, decoded.Path)
		})
	}
}

func TestDecodeCursor_EmptyString(t *testing.T) {
	t.Parallel()

	cursor, err := internal.DecodeCursor("")
	require.NoError(t, err)

	assert.True(t, cursor.CreatedAt.IsZero(), "empty cursor should return zero time")
	assert.Empty(t, cursor.Path, "empty cursor should return empty path")
}

func TestDecodeCursor_InvalidBase64(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		cursor string
	}{
		{
			name:   "not base64",
			cursor: "not-valid-base64!!!",
		},
		{
			name:   "wrong padding",
			cursor: "aGVsbG8===",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := internal.DecodeCursor(tt.cursor)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "invalid encoding")
		})
	}
}

func TestDecodeCursor_InvalidFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		rawData     string
		errContains string
	}{
		{
			name:        "missing pipe separator",
			rawData:     "2024-01-15T10:30:00Z",
			errContains: "invalid format",
		},
		{
			name:        "empty path after pipe",
			rawData:     "2024-01-15T10:30:00Z|",
			errContains: "empty path",
		},
		{
			name:        "invalid timestamp format",
			rawData:     "not-a-timestamp|file.txt",
			errContains: "invalid timestamp",
		},
		{
			name:        "wrong timestamp format",
			rawData:     "2024/01/15 10:30:00|file.txt",
			errContains: "invalid timestamp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			encoded := base64.URLEncoding.EncodeToString([]byte(tt.rawData))

			_, err := internal.DecodeCursor(encoded)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}
}

func TestEscapeLikePattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no special characters",
			input:    "simple/path/file.txt",
			expected: "simple/path/file.txt",
		},
		{
			name:     "percent sign",
			input:    "100%complete",
			expected: `100\%complete`,
		},
		{
			name:     "underscore",
			input:    "file_name.txt",
			expected: `file\_name.txt`,
		},
		{
			name:     "backslash",
			input:    `path\to\file`,
			expected: `path\\to\\file`,
		},
		{
			name:     "all special characters",
			input:    `50%_done\today`,
			expected: `50\%\_done\\today`,
		},
		{
			name:     "multiple consecutive special chars",
			input:    "%%__\\\\",
			expected: `\%\%\_\_\\\\`,
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only special characters",
			input:    `%_\`,
			expected: `\%\_\\`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := internal.EscapeLikePattern(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
