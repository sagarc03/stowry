package database

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCursorRoundTrip(t *testing.T) {
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
			// The separator is a pipe, so this is the case a naive split breaks.
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

			encoded := encodeCursor(tt.createdAt, tt.path)
			assert.NotEmpty(t, encoded)

			decoded, err := decodeCursor(encoded)
			require.NoError(t, err)

			assert.True(t, tt.createdAt.Equal(decoded.CreatedAt),
				"createdAt mismatch: expected %v, got %v", tt.createdAt, decoded.CreatedAt)
			assert.Equal(t, tt.path, decoded.Path)
		})
	}
}

func TestDecodeCursorEmpty(t *testing.T) {
	t.Parallel()

	c, err := decodeCursor("")
	require.NoError(t, err)

	assert.True(t, c.CreatedAt.IsZero())
	assert.Empty(t, c.Path)
}

func TestDecodeCursorRejectsMalformed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		token       string
		errContains string
	}{
		{name: "not base64", token: "not-valid-base64!!!", errContains: "invalid encoding"},
		{name: "wrong padding", token: "aGVsbG8===", errContains: "invalid encoding"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := decodeCursor(tt.token)
			assert.ErrorContains(t, err, tt.errContains)
		})
	}
}

func TestDecodeCursorRejectsBadPayload(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		payload     string
		errContains string
	}{
		{name: "missing pipe separator", payload: "2024-01-15T10:30:00Z", errContains: "invalid format"},
		{name: "empty path after pipe", payload: "2024-01-15T10:30:00Z|", errContains: "empty path"},
		{name: "invalid timestamp", payload: "not-a-timestamp|file.txt", errContains: "invalid timestamp"},
		{name: "wrong timestamp layout", payload: "2024/01/15 10:30:00|file.txt", errContains: "invalid timestamp"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := decodeCursor(base64.URLEncoding.EncodeToString([]byte(tt.payload)))
			assert.ErrorContains(t, err, tt.errContains)
		})
	}
}

func TestEscapeLikePattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "no special characters", input: "simple/path/file.txt", want: "simple/path/file.txt"},
		{name: "percent sign", input: "100%complete", want: `100\%complete`},
		{name: "underscore", input: "file_name.txt", want: `file\_name.txt`},
		{name: "backslash", input: `path\to\file`, want: `path\\to\\file`},
		{name: "all special characters", input: `50%_done\today`, want: `50\%\_done\\today`},
		{name: "consecutive special characters", input: `%%__\\`, want: `\%\%\_\_\\\\`},
		{name: "empty string", input: "", want: ""},
		{name: "only special characters", input: `%_\`, want: `\%\_\\`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, escapeLikePattern(tt.input))
		})
	}
}
