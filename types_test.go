package stowry_test

import (
	"testing"

	"github.com/sagarc03/stowry"
	"github.com/stretchr/testify/assert"
)

func TestServerMode_IsValid(t *testing.T) {
	tests := []struct {
		name  string
		mode  stowry.ServerMode
		valid bool
	}{
		{
			name:  "store mode is valid",
			mode:  stowry.ModeStore,
			valid: true,
		},
		{
			name:  "static mode is valid",
			mode:  stowry.ModeStatic,
			valid: true,
		},
		{
			name:  "spa mode is valid",
			mode:  stowry.ModeSPA,
			valid: true,
		},
		{
			name:  "empty mode is invalid",
			mode:  "",
			valid: false,
		},
		{
			name:  "random string is invalid",
			mode:  "invalid",
			valid: false,
		},
		{
			name:  "uppercase mode is invalid",
			mode:  "STORE",
			valid: false,
		},
		{
			name:  "mixed case mode is invalid",
			mode:  "Store",
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.valid, tt.mode.IsValid())
		})
	}
}

func TestParseServerMode(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantMode  stowry.ServerMode
		wantError bool
	}{
		{
			name:      "parse store mode",
			input:     "store",
			wantMode:  stowry.ModeStore,
			wantError: false,
		},
		{
			name:      "parse static mode",
			input:     "static",
			wantMode:  stowry.ModeStatic,
			wantError: false,
		},
		{
			name:      "parse spa mode",
			input:     "spa",
			wantMode:  stowry.ModeSPA,
			wantError: false,
		},
		{
			name:      "empty string returns error",
			input:     "",
			wantMode:  "",
			wantError: true,
		},
		{
			name:      "invalid mode returns error",
			input:     "invalid",
			wantMode:  "",
			wantError: true,
		},
		{
			name:      "uppercase mode returns error",
			input:     "STORE",
			wantMode:  "",
			wantError: true,
		},
		{
			name:      "mixed case mode returns error",
			input:     "Static",
			wantMode:  "",
			wantError: true,
		},
		{
			name:      "server mode returns error",
			input:     "server",
			wantMode:  "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mode, err := stowry.ParseServerMode(tt.input)

			if tt.wantError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "invalid server mode")
				assert.Contains(t, err.Error(), tt.input)
				assert.Equal(t, tt.wantMode, mode)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantMode, mode)
			}
		})
	}
}

func TestServerMode_Constants(t *testing.T) {
	t.Run("mode constants have expected values", func(t *testing.T) {
		assert.Equal(t, stowry.ServerMode("store"), stowry.ModeStore)
		assert.Equal(t, stowry.ServerMode("static"), stowry.ModeStatic)
		assert.Equal(t, stowry.ServerMode("spa"), stowry.ModeSPA)
	})

	t.Run("mode constants are all valid", func(t *testing.T) {
		assert.True(t, stowry.ModeStore.IsValid())
		assert.True(t, stowry.ModeStatic.IsValid())
		assert.True(t, stowry.ModeSPA.IsValid())
	})
}
