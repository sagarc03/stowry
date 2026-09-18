package sign

import "errors"

// Sentinel errors returned by Verify.
var (
	ErrMissingParams = errors.New("missing required signature parameters")
	// ErrExpired means the URL was signed correctly but timestamp+expires has passed.
	ErrExpired = errors.New("signature expired")
	// ErrInvalidCredential means the access key is not in the SecretStore.
	ErrInvalidCredential = errors.New("invalid credential")
	// ErrInvalidSignature means tampering or a mismatched key.
	ErrInvalidSignature = errors.New("invalid signature")
)
