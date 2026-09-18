// Package sign signs and verifies presigned URLs, in Stowry's own scheme and
// AWS Signature V4.
package sign

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// Validity periods, in seconds.
const (
	DefaultExpires = 900    // 15 minutes
	MaxExpires     = 604800 // 7 days
)

// Query parameters carrying a Stowry signature.
const (
	StowryCredentialParam = "X-Stowry-Credential" //nolint:gosec // parameter name, not a credential
	StowryDateParam       = "X-Stowry-Date"
	StowryExpiresParam    = "X-Stowry-Expires"
	StowrySignatureParam  = "X-Stowry-Signature"
)

// Sign returns the hex-encoded HMAC-SHA256 of
//
//	{METHOD}\n{PATH}\n{TIMESTAMP}\n{EXPIRES}
func Sign(secretKey, method, path string, timestamp, expires int64) string {
	stringToSign := fmt.Sprintf("%s\n%s\n%d\n%d", method, path, timestamp, expires)
	h := hmac.New(sha256.New, []byte(secretKey))
	h.Write([]byte(stringToSign))
	return hex.EncodeToString(h.Sum(nil))
}
