package stowry_test

import (
	"testing"
	"unicode/utf8"

	"github.com/sagarc03/stowry"
)

func TestIsValidPath(t *testing.T) {
	// Create a path with invalid UTF-8 (without embedding raw invalid bytes in source)
	invalidUTF8 := string([]byte{'/', 'a', 0xff, 'b'})

	tt := []struct {
		Name string
		Path string
		Want bool
	}{
		// Basics
		{Name: "root path", Path: "/", Want: false},
		{Name: "empty path", Path: "", Want: false},
		{Name: "no leading slash", Path: "/some/path", Want: false},
		{Name: "ends with slash", Path: "some/path/", Want: false},

		// Double dots anywhere are invalid
		{Name: "double dots segment", Path: "../", Want: false},
		{Name: "double dots in middle segment", Path: "a/../b", Want: false},
		{Name: "double dots at end", Path: "/a/..", Want: false},
		{Name: "double dots in filename", Path: "/a/b..c", Want: false},
		{Name: "double dots prefix", Path: "/a/..b", Want: false},

		// Signle dots segment are invalid
		{Name: "single dot segment not allowed", Path: "a/./b", Want: false},
		{Name: "single dot only", Path: ".", Want: false},

		// Double slashes invalid
		{Name: "double slash", Path: "a//b", Want: false},
		{Name: "leading double slash", Path: "//a", Want: false},

		// Forbidden characters
		{Name: "contains space", Path: "some path/file.ext", Want: false},
		{Name: "contains tab", Path: "some\tpath/file.ext", Want: false},
		{Name: "contains newline", Path: "some\npath/file.ext", Want: false},
		{Name: "contains carriage return", Path: "some\rpath/file.ext", Want: false},
		{Name: "contains backslash", Path: `some\path/file.ext`, Want: false},
		{Name: "contains hash", Path: "some/path#frag", Want: false},
		{Name: "contains question mark", Path: "some/path?x=1", Want: false},
		{Name: "contains tilde", Path: "some/~path/file.ext", Want: false},

		// Control chars / NUL
		{Name: "contains NUL", Path: "some\x00path/file.ext", Want: false},
		{Name: "contains DEL", Path: "some\x7fpath/file.ext", Want: false},
		{Name: "contains control char", Path: "some\x1fpath/file.ext", Want: false},

		// UTF-8 validity
		{Name: "invalid utf8", Path: invalidUTF8, Want: false},

		// Valid examples
		{Name: "simple valid", Path: "some/path/file.ext", Want: true},
		{Name: "hidden file valid", Path: ".hidden/file", Want: true},
		{Name: "underscores and dashes valid", Path: "some_path/with-dash/file_name.ext", Want: true},
		{Name: "percent is allowed as literal", Path: "a/%2e/b", Want: true}, // you didn't ban '%'
		{Name: "unicode valid", Path: "привет/世界/file.ext", Want: true},
	}

	// sanity check for our generated invalid UTF-8 case
	if utf8.ValidString(invalidUTF8) {
		t.Fatalf("test setup error: invalidUTF8 is unexpectedly valid")
	}

	for _, tc := range tt {
		t.Run(tc.Name, func(t *testing.T) {
			got := stowry.IsValidPath(tc.Path)
			if got != tc.Want {
				expected := "valid"
				if !tc.Want {
					expected = "invalid"
				}
				t.Errorf("expected path %q to be %s, got %v", tc.Path, expected, got)
			}
		})
	}
}
