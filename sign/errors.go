package sign

import "errors"

// Sentinel errors returned by [Verifier.Verify].
var (
	// ErrMissingParams is returned when required query parameters are missing
	// or malformed. Required parameters: X-Stowry-Credential, X-Stowry-Date,
	// X-Stowry-Expires, X-Stowry-Signature.
	ErrMissingParams = errors.New("missing required signature parameters")

	// ErrExpired is returned when the signature validity period has elapsed.
	// The URL was valid but the current time exceeds timestamp + expires.
	ErrExpired = errors.New("signature expired")

	// ErrInvalidCredential is returned when the access key is not recognized
	// by the lookup function provided to [NewVerifier].
	ErrInvalidCredential = errors.New("invalid credential")

	// ErrInvalidSignature is returned when the signature does not match the
	// expected value. This indicates either tampering or mismatched keys.
	ErrInvalidSignature = errors.New("invalid signature")
)
