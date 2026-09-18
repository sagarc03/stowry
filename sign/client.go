package sign

import (
	"fmt"
	"strings"
	"time"
)

// Client generates presigned URLs for Stowry.
//
// A Client is safe for concurrent use by multiple goroutines.
type Client struct {
	endpoint  string
	accessKey string
	secretKey string
}

// NewClient creates a Client for generating presigned URLs.
//
// The endpoint is the base URL of your Stowry server (e.g., "https://storage.example.com").
// Trailing slashes are automatically trimmed.
func NewClient(endpoint, accessKey, secretKey string) *Client {
	return &Client{
		endpoint:  strings.TrimSuffix(endpoint, "/"),
		accessKey: accessKey,
		secretKey: secretKey,
	}
}

// PresignGet generates a presigned URL for downloading an object.
//
// The path should include any bucket or prefix (e.g., "/bucket/path/to/file.pdf").
// Paths without a leading slash are automatically normalized.
//
// The expires parameter specifies the validity period in seconds. Values <= 0
// default to [DefaultExpires] (900 seconds). Values exceeding [MaxExpires]
// (604800 seconds) are capped.
func (c *Client) PresignGet(path string, expires int) string {
	return c.preSign("GET", path, expires)
}

// PresignPut generates a presigned URL for uploading an object.
//
// See [Client.PresignGet] for parameter details.
func (c *Client) PresignPut(path string, expires int) string {
	return c.preSign("PUT", path, expires)
}

// PresignDelete generates a presigned URL for deleting an object.
//
// See [Client.PresignGet] for parameter details.
func (c *Client) PresignDelete(path string, expires int) string {
	return c.preSign("DELETE", path, expires)
}

func (c *Client) preSign(method, path string, expires int) string {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	if expires <= 0 {
		expires = DefaultExpires
	}
	if expires > MaxExpires {
		expires = MaxExpires
	}

	timestamp := time.Now().Unix()
	signature := Sign(c.secretKey, method, path, timestamp, int64(expires))

	// Query params sorted alphabetically
	return fmt.Sprintf("%s%s?%s=%s&%s=%d&%s=%d&%s=%s",
		c.endpoint, path,
		StowryCredentialParam, c.accessKey,
		StowryDateParam, timestamp,
		StowryExpiresParam, expires,
		StowrySignatureParam, signature,
	)
}
