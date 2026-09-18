package sign

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Client generates presigned URLs. It is safe for concurrent use.
type Client struct {
	endpoint  string
	accessKey string
	secretKey string
}

// NewClient returns a Client signing for endpoint, the server's base URL. A
// trailing slash is trimmed.
func NewClient(endpoint, accessKey, secretKey string) *Client {
	return &Client{
		endpoint:  strings.TrimSuffix(endpoint, "/"),
		accessKey: accessKey,
		secretKey: secretKey,
	}
}

// PresignGet returns a presigned URL for downloading path. A missing leading
// slash is added. expires is in seconds: zero or less means [DefaultExpires],
// and anything above [MaxExpires] is capped.
func (c *Client) PresignGet(path string, expires int) string {
	return c.preSign("GET", path, expires)
}

// PresignPut returns a presigned URL for uploading path.
// See [Client.PresignGet] for the parameters.
func (c *Client) PresignPut(path string, expires int) string {
	return c.preSign("PUT", path, expires)
}

// PresignDelete returns a presigned URL for deleting path.
// See [Client.PresignGet] for the parameters.
func (c *Client) PresignDelete(path string, expires int) string {
	return c.preSign("DELETE", path, expires)
}

// PresignList returns a presigned URL for listing objects. The signature covers
// the root path only, so prefix, limit and cursor are appended after signing. A
// limit of zero is omitted, leaving the server its own default.
func (c *Client) PresignList(prefix string, limit int, cursor string, expires int) string {
	signed := c.preSign("GET", "/", expires)

	extra := url.Values{}
	if prefix != "" {
		extra.Set("prefix", prefix)
	}
	if limit > 0 {
		extra.Set("limit", strconv.Itoa(limit))
	}
	if cursor != "" {
		extra.Set("cursor", cursor)
	}

	if len(extra) == 0 {
		return signed
	}

	return signed + "&" + extra.Encode()
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

	return fmt.Sprintf("%s%s?%s=%s&%s=%d&%s=%d&%s=%s",
		c.endpoint, path,
		StowryCredentialParam, c.accessKey,
		StowryDateParam, timestamp,
		StowryExpiresParam, expires,
		StowrySignatureParam, signature,
	)
}
