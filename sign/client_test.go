package sign

import (
	"strings"
	"testing"
)

func TestSign(t *testing.T) {
	secretKey := "9218d0ddfdb1779169f4b6b3b36df321099e98e9"
	method := "GET"
	path := "/test/hello.txt"
	timestamp := int64(1736956800)
	expires := int64(900)
	expected := "b24285352583edb3d06c531f61e38c5706d42d79e31474bf1f95667d524bae21"

	got := Sign(secretKey, method, path, timestamp, expires)
	if got != expected {
		t.Errorf("Sign() = %q, want %q", got, expected)
	}
}

func TestNewClient(t *testing.T) {
	c := NewClient("http://localhost:5708/", "access", "secret")
	if c.endpoint != "http://localhost:5708" {
		t.Errorf("endpoint = %q, want trailing slash trimmed", c.endpoint)
	}
	if c.accessKey != "access" {
		t.Errorf("accessKey = %q, want %q", c.accessKey, "access")
	}
	if c.secretKey != "secret" {
		t.Errorf("secretKey = %q, want %q", c.secretKey, "secret")
	}
}

func TestClientPresignGet(t *testing.T) {
	c := NewClient("http://localhost:5708", "FE373CEF5632FDED3081", "secret")
	url := c.PresignGet("/files/doc.pdf", 900)

	if !strings.HasPrefix(url, "http://localhost:5708/files/doc.pdf?") {
		t.Errorf("URL should start with endpoint and path, got %q", url)
	}
	if !strings.Contains(url, "X-Stowry-Credential=FE373CEF5632FDED3081") {
		t.Errorf("URL should contain credential, got %q", url)
	}
	if !strings.Contains(url, "X-Stowry-Expires=900") {
		t.Errorf("URL should contain expires, got %q", url)
	}
	if !strings.Contains(url, "X-Stowry-Date=") {
		t.Errorf("URL should contain date, got %q", url)
	}
	if !strings.Contains(url, "X-Stowry-Signature=") {
		t.Errorf("URL should contain signature, got %q", url)
	}
}

func TestClientPresignPut(t *testing.T) {
	c := NewClient("http://localhost:5708", "access", "secret")
	url := c.PresignPut("/files/upload.txt", 900)

	if !strings.HasPrefix(url, "http://localhost:5708/files/upload.txt?") {
		t.Errorf("URL should start with endpoint and path, got %q", url)
	}
}

func TestClientPresignDelete(t *testing.T) {
	c := NewClient("http://localhost:5708", "access", "secret")
	url := c.PresignDelete("/files/old.txt", 900)

	if !strings.HasPrefix(url, "http://localhost:5708/files/old.txt?") {
		t.Errorf("URL should start with endpoint and path, got %q", url)
	}
}

func TestClientPathNormalization(t *testing.T) {
	c := NewClient("http://localhost:5708", "access", "secret")

	url := c.PresignGet("no-leading-slash", 900)
	if !strings.Contains(url, "/no-leading-slash?") {
		t.Errorf("path should be normalized with leading slash, got %q", url)
	}
}

func TestClientExpiresDefaults(t *testing.T) {
	c := NewClient("http://localhost:5708", "access", "secret")

	url := c.PresignGet("/test", 0)
	if !strings.Contains(url, "X-Stowry-Expires=900") {
		t.Errorf("expires=0 should default to 900, got %q", url)
	}

	url = c.PresignGet("/test", -1)
	if !strings.Contains(url, "X-Stowry-Expires=900") {
		t.Errorf("expires=-1 should default to 900, got %q", url)
	}
}

func TestClientExpiresMax(t *testing.T) {
	c := NewClient("http://localhost:5708", "access", "secret")

	url := c.PresignGet("/test", 999999)
	if !strings.Contains(url, "X-Stowry-Expires=604800") {
		t.Errorf("expires should be capped at MaxExpires, got %q", url)
	}
}
