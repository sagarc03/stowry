package types_test

import (
	"strings"
	"testing"

	"github.com/sagarc03/stowry/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerModeIsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode types.ServerMode
		want bool
	}{
		{name: "store", mode: types.ModeStore, want: true},
		{name: "static", mode: types.ModeStatic, want: true},
		{name: "spa", mode: types.ModeSPA, want: true},
		{name: "empty", mode: "", want: false},
		{name: "unknown", mode: "invalid", want: false},
		{name: "uppercase", mode: "STORE", want: false},
		{name: "mixed case", mode: "Store", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.mode.IsValid())
		})
	}
}

func TestParseServerMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  types.ServerMode
	}{
		{name: "store", input: "store", want: types.ModeStore},
		{name: "static", input: "static", want: types.ModeStatic},
		{name: "spa", input: "spa", want: types.ModeSPA},
		{name: "empty", input: ""},
		{name: "unknown", input: "invalid"},
		{name: "uppercase", input: "STORE"},
		{name: "mixed case", input: "Static"},
		{name: "server is not a mode", input: "server"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mode, err := types.ParseServerMode(tt.input)

			if tt.want == "" {
				assert.ErrorContains(t, err, "invalid server mode")
				assert.ErrorContains(t, err, tt.input)
				assert.Empty(t, mode)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, mode)
		})
	}
}

func TestIsValidTableName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		table string
		want  bool
	}{
		{name: "lowercase word", table: "metadata", want: true},
		{name: "underscores and digits", table: "stowry_metadata_2", want: true},
		{name: "leading underscore", table: "_metadata", want: true},
		{name: "63 characters", table: "a" + strings.Repeat("b", 62), want: true},
		{name: "64 characters", table: "a" + strings.Repeat("b", 63), want: false},
		{name: "empty", table: "", want: false},
		{name: "leading digit", table: "1metadata", want: false},
		{name: "uppercase", table: "MetaData", want: false},
		{name: "hyphen", table: "meta-data", want: false},
		{name: "space", table: "meta data", want: false},
		{name: "quote", table: `meta"data`, want: false},
		{name: "statement separator", table: "meta; DROP TABLE x", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, types.IsValidTableName(tt.table))
		})
	}
}

func TestTablesValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		tables      types.Tables
		errContains string
	}{
		{name: "valid", tables: types.Tables{MetaData: "stowry_metadata"}},
		{name: "empty", tables: types.Tables{}, errContains: "cannot be empty"},
		{name: "not an identifier", tables: types.Tables{MetaData: "Meta-Data"}, errContains: "invalid metadata table name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.tables.Validate()
			if tt.errContains == "" {
				assert.NoError(t, err)
				return
			}

			assert.ErrorContains(t, err, tt.errContains)
		})
	}
}
