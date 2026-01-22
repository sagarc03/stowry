package stowry

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// IsValidPath validates that a path string meets the requirements for a storage path.
// It checks that the path:
//   - is not empty, ".", or "/"
//   - is relative (does not start with "/")
//   - does not end with "/"
//   - does not contain ".." (path traversal)
//   - does not contain "//" (empty segments)
//   - does not contain invalid characters: \ ? # ~
//   - is valid UTF-8
//   - does not contain "." segments (/., /./, or ending with /.)
//   - does not contain null bytes, control characters (< 0x20), DEL (0x7f), or whitespace
//
// Returns true if the path is valid, false otherwise.
func IsValidPath(p string) bool {
	if p == "" || p == "/" || p == "." {
		return false
	}

	if p[0] == '/' {
		return false
	}

	if strings.HasSuffix(p, "/") {
		return false
	}

	if strings.Contains(p, "..") {
		return false
	}

	if strings.Contains(p, "//") {
		return false
	}

	if strings.ContainsAny(p, `\?#~`) {
		return false
	}

	if !utf8.ValidString(p) {
		return false
	}

	if p == "/." || strings.Contains(p, "/./") || strings.HasSuffix(p, "/.") {
		return false
	}

	for _, r := range p {
		if r == 0 || r < 0x20 || r == 0x7f || unicode.IsSpace(r) {
			return false
		}
	}

	return true
}
