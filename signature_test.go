package stowry_test

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/sagarc03/stowry"
	stowrysign "github.com/sagarc03/stowry-go"
	"github.com/sagarc03/stowry/keybackend"
	"github.com/stretchr/testify/assert"
)

func TestAWSSignatureVerifier_Verify(t *testing.T) {
	store := keybackend.NewMapSecretStore(map[string]string{
		"AKIATEST": "testsecret",
	})

	verifier := stowry.NewAWSSignatureVerifier("us-east-1", "s3", store)

	validTime := time.Now().UTC().Add(-30 * time.Minute)
	validDateStamp := validTime.Format(stowry.DateFormat)
	validAmzDate := validTime.Format(stowry.DateTimeFormat)

	oldTime := time.Now().Add(-2 * time.Hour)
	oldDateStamp := oldTime.Format(stowry.DateFormat)
	oldAmzDate := oldTime.Format(stowry.DateTimeFormat)

	tests := []struct {
		name      string
		query     url.Values
		wantError string
	}{
		{
			name:      "empty query",
			query:     url.Values{},
			wantError: "missing required signature parameters",
		},
		{
			name: "missing algorithm",
			query: url.Values{
				"X-Amz-Credential":    []string{"AKIATEST/20260112/us-east-1/s3/aws4_request"},
				"X-Amz-Date":          []string{"20260112T070000Z"},
				"X-Amz-Expires":       []string{"3600"},
				"X-Amz-SignedHeaders": []string{"host"},
				"X-Amz-Signature":     []string{"abc123"},
			},
			wantError: "missing required signature parameters",
		},
		{
			name: "invalid algorithm",
			query: url.Values{
				"X-Amz-Algorithm":     []string{"AWS4-HMAC-SHA1"},
				"X-Amz-Credential":    []string{"AKIATEST/20260112/us-east-1/s3/aws4_request"},
				"X-Amz-Date":          []string{"20260112T070000Z"},
				"X-Amz-Expires":       []string{"3600"},
				"X-Amz-SignedHeaders": []string{"host"},
				"X-Amz-Signature":     []string{"abc123"},
			},
			wantError: "invalid algorithm",
		},
		{
			name: "invalid date format",
			query: url.Values{
				"X-Amz-Algorithm":     []string{"AWS4-HMAC-SHA256"},
				"X-Amz-Credential":    []string{"AKIATEST/20260112/us-east-1/s3/aws4_request"},
				"X-Amz-Date":          []string{"invalid-date"},
				"X-Amz-Expires":       []string{"3600"},
				"X-Amz-SignedHeaders": []string{"host"},
				"X-Amz-Signature":     []string{"abc123"},
			},
			wantError: "invalid X-Amz-Date format",
		},
		{
			name: "expires zero",
			query: url.Values{
				"X-Amz-Algorithm":     []string{"AWS4-HMAC-SHA256"},
				"X-Amz-Credential":    []string{"AKIATEST/20260112/us-east-1/s3/aws4_request"},
				"X-Amz-Date":          []string{"20260112T070000Z"},
				"X-Amz-Expires":       []string{"0"},
				"X-Amz-SignedHeaders": []string{"host"},
				"X-Amz-Signature":     []string{"abc123"},
			},
			wantError: "invalid X-Amz-Expires",
		},
		{
			name: "expires too large",
			query: url.Values{
				"X-Amz-Algorithm":     []string{"AWS4-HMAC-SHA256"},
				"X-Amz-Credential":    []string{"AKIATEST/20260112/us-east-1/s3/aws4_request"},
				"X-Amz-Date":          []string{"20260112T070000Z"},
				"X-Amz-Expires":       []string{"604801"},
				"X-Amz-SignedHeaders": []string{"host"},
				"X-Amz-Signature":     []string{"abc123"},
			},
			wantError: "invalid X-Amz-Expires",
		},
		{
			name: "expired signature",
			query: url.Values{
				"X-Amz-Algorithm":     []string{"AWS4-HMAC-SHA256"},
				"X-Amz-Credential":    []string{fmt.Sprintf("AKIATEST/%s/us-east-1/s3/aws4_request", oldDateStamp)},
				"X-Amz-Date":          []string{oldAmzDate},
				"X-Amz-Expires":       []string{"3600"},
				"X-Amz-SignedHeaders": []string{"host"},
				"X-Amz-Signature":     []string{"abc123"},
			},
			wantError: "signature expired",
		},
		{
			name: "invalid credential format",
			query: url.Values{
				"X-Amz-Algorithm":     []string{"AWS4-HMAC-SHA256"},
				"X-Amz-Credential":    []string{"AKIATEST/invalid"},
				"X-Amz-Date":          []string{validAmzDate},
				"X-Amz-Expires":       []string{"3600"},
				"X-Amz-SignedHeaders": []string{"host"},
				"X-Amz-Signature":     []string{"abc123"},
			},
			wantError: "invalid X-Amz-Credential format",
		},
		{
			name: "invalid access key",
			query: url.Values{
				"X-Amz-Algorithm":     []string{"AWS4-HMAC-SHA256"},
				"X-Amz-Credential":    []string{fmt.Sprintf("WRONGKEY/%s/us-east-1/s3/aws4_request", validDateStamp)},
				"X-Amz-Date":          []string{validAmzDate},
				"X-Amz-Expires":       []string{"3600"},
				"X-Amz-SignedHeaders": []string{"host"},
				"X-Amz-Signature":     []string{"abc123"},
			},
			wantError: "access key not found",
		},
		{
			name: "region mismatch",
			query: url.Values{
				"X-Amz-Algorithm":     []string{"AWS4-HMAC-SHA256"},
				"X-Amz-Credential":    []string{fmt.Sprintf("AKIATEST/%s/us-west-2/s3/aws4_request", validDateStamp)},
				"X-Amz-Date":          []string{validAmzDate},
				"X-Amz-Expires":       []string{"3600"},
				"X-Amz-SignedHeaders": []string{"host"},
				"X-Amz-Signature":     []string{"abc123"},
			},
			wantError: "region mismatch",
		},
		{
			name: "service mismatch",
			query: url.Values{
				"X-Amz-Algorithm":     []string{"AWS4-HMAC-SHA256"},
				"X-Amz-Credential":    []string{fmt.Sprintf("AKIATEST/%s/us-east-1/ec2/aws4_request", validDateStamp)},
				"X-Amz-Date":          []string{validAmzDate},
				"X-Amz-Expires":       []string{"3600"},
				"X-Amz-SignedHeaders": []string{"host"},
				"X-Amz-Signature":     []string{"abc123"},
			},
			wantError: "service mismatch",
		},
		{
			name: "invalid terminator",
			query: url.Values{
				"X-Amz-Algorithm":     []string{"AWS4-HMAC-SHA256"},
				"X-Amz-Credential":    []string{fmt.Sprintf("AKIATEST/%s/us-east-1/s3/wrong", validDateStamp)},
				"X-Amz-Date":          []string{validAmzDate},
				"X-Amz-Expires":       []string{"3600"},
				"X-Amz-SignedHeaders": []string{"host"},
				"X-Amz-Signature":     []string{"abc123"},
			},
			wantError: "invalid credential terminator",
		},
		{
			name: "credential date mismatch",
			query: url.Values{
				"X-Amz-Algorithm":     []string{"AWS4-HMAC-SHA256"},
				"X-Amz-Credential":    []string{"AKIATEST/20260101/us-east-1/s3/aws4_request"},
				"X-Amz-Date":          []string{validAmzDate},
				"X-Amz-Expires":       []string{"3600"},
				"X-Amz-SignedHeaders": []string{"host"},
				"X-Amz-Signature":     []string{"abc123"},
			},
			wantError: "credential date mismatch",
		},
		{
			name: "signature mismatch",
			query: url.Values{
				"X-Amz-Algorithm":     []string{"AWS4-HMAC-SHA256"},
				"X-Amz-Credential":    []string{fmt.Sprintf("AKIATEST/%s/us-east-1/s3/aws4_request", validDateStamp)},
				"X-Amz-Date":          []string{validAmzDate},
				"X-Amz-Expires":       []string{"3600"},
				"X-Amz-SignedHeaders": []string{"host"},
				"X-Amz-Signature":     []string{"wrongsignature123"},
			},
			wantError: "signature mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqURL := &url.URL{
				Path:     "/test.txt",
				RawQuery: tt.query.Encode(),
			}
			req := &http.Request{
				Method: "GET",
				URL:    reqURL,
				Host:   "localhost:5708",
				Header: http.Header{},
			}
			err := verifier.Verify(req)

			if tt.wantError == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantError)
			}
		})
	}
}

func TestNewAWSSignatureVerifier(t *testing.T) {
	store := keybackend.NewMapSecretStore(map[string]string{
		"test": "secret",
	})

	verifier := stowry.NewAWSSignatureVerifier("us-west-1", "ec2", store)

	assert.NotNil(t, verifier)
	assert.Equal(t, "us-west-1", verifier.Region)
	assert.Equal(t, "ec2", verifier.Service)
}

func TestNewSignatureVerifier(t *testing.T) {
	store := keybackend.NewMapSecretStore(map[string]string{
		"test": "secret",
	})

	verifier := stowry.NewSignatureVerifier("us-west-1", "ec2", store)
	assert.NotNil(t, verifier)
}

func TestStowrySignatureVerifier_Verify(t *testing.T) {
	const (
		accessKey = "STOWRYTEST"
		secretKey = "testsecret123"
	)

	store := keybackend.NewMapSecretStore(map[string]string{
		accessKey: secretKey,
	})

	verifier := stowry.NewStowrySignatureVerifier(store)

	validTimestamp := time.Now().Unix()
	validExpires := int64(900)
	validSignature := stowrysign.Sign(secretKey, "GET", "/test.txt", validTimestamp, validExpires)

	expiredTimestamp := time.Now().Add(-2 * time.Hour).Unix()
	expiredSignature := stowrysign.Sign(secretKey, "GET", "/test.txt", expiredTimestamp, validExpires)

	tests := []struct {
		name      string
		query     url.Values
		wantError string
	}{
		{
			name:      "empty query",
			query:     url.Values{},
			wantError: "missing required signature parameters",
		},
		{
			name: "missing credential",
			query: url.Values{
				"X-Stowry-Date":      []string{fmt.Sprintf("%d", validTimestamp)},
				"X-Stowry-Expires":   []string{fmt.Sprintf("%d", validExpires)},
				"X-Stowry-Signature": []string{validSignature},
			},
			wantError: "missing required signature parameters",
		},
		{
			name: "missing date",
			query: url.Values{
				"X-Stowry-Credential": []string{accessKey},
				"X-Stowry-Expires":    []string{fmt.Sprintf("%d", validExpires)},
				"X-Stowry-Signature":  []string{validSignature},
			},
			wantError: "missing required signature parameters",
		},
		{
			name: "missing expires",
			query: url.Values{
				"X-Stowry-Credential": []string{accessKey},
				"X-Stowry-Date":       []string{fmt.Sprintf("%d", validTimestamp)},
				"X-Stowry-Signature":  []string{validSignature},
			},
			wantError: "missing required signature parameters",
		},
		{
			name: "missing signature",
			query: url.Values{
				"X-Stowry-Credential": []string{accessKey},
				"X-Stowry-Date":       []string{fmt.Sprintf("%d", validTimestamp)},
				"X-Stowry-Expires":    []string{fmt.Sprintf("%d", validExpires)},
			},
			wantError: "missing required signature parameters",
		},
		{
			name: "expires zero",
			query: url.Values{
				"X-Stowry-Credential": []string{accessKey},
				"X-Stowry-Date":       []string{fmt.Sprintf("%d", validTimestamp)},
				"X-Stowry-Expires":    []string{"0"},
				"X-Stowry-Signature":  []string{validSignature},
			},
			wantError: "invalid expires",
		},
		{
			name: "expires negative",
			query: url.Values{
				"X-Stowry-Credential": []string{accessKey},
				"X-Stowry-Date":       []string{fmt.Sprintf("%d", validTimestamp)},
				"X-Stowry-Expires":    []string{"-1"},
				"X-Stowry-Signature":  []string{validSignature},
			},
			wantError: "invalid expires",
		},
		{
			name: "expires too large",
			query: url.Values{
				"X-Stowry-Credential": []string{accessKey},
				"X-Stowry-Date":       []string{fmt.Sprintf("%d", validTimestamp)},
				"X-Stowry-Expires":    []string{"604801"},
				"X-Stowry-Signature":  []string{validSignature},
			},
			wantError: "invalid expires",
		},
		{
			name: "expired signature",
			query: url.Values{
				"X-Stowry-Credential": []string{accessKey},
				"X-Stowry-Date":       []string{fmt.Sprintf("%d", expiredTimestamp)},
				"X-Stowry-Expires":    []string{fmt.Sprintf("%d", validExpires)},
				"X-Stowry-Signature":  []string{expiredSignature},
			},
			wantError: "signature expired",
		},
		{
			name: "access key not found",
			query: url.Values{
				"X-Stowry-Credential": []string{"WRONGKEY"},
				"X-Stowry-Date":       []string{fmt.Sprintf("%d", validTimestamp)},
				"X-Stowry-Expires":    []string{fmt.Sprintf("%d", validExpires)},
				"X-Stowry-Signature":  []string{validSignature},
			},
			wantError: "access key not found",
		},
		{
			name: "signature mismatch",
			query: url.Values{
				"X-Stowry-Credential": []string{accessKey},
				"X-Stowry-Date":       []string{fmt.Sprintf("%d", validTimestamp)},
				"X-Stowry-Expires":    []string{fmt.Sprintf("%d", validExpires)},
				"X-Stowry-Signature":  []string{"wrongsignature123"},
			},
			wantError: "signature mismatch",
		},
		{
			name: "valid signature",
			query: url.Values{
				"X-Stowry-Credential": []string{accessKey},
				"X-Stowry-Date":       []string{fmt.Sprintf("%d", validTimestamp)},
				"X-Stowry-Expires":    []string{fmt.Sprintf("%d", validExpires)},
				"X-Stowry-Signature":  []string{validSignature},
			},
			wantError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqURL := &url.URL{
				Path:     "/test.txt",
				RawQuery: tt.query.Encode(),
			}
			req := &http.Request{
				Method: "GET",
				URL:    reqURL,
				Host:   "localhost:5708",
				Header: http.Header{},
			}
			err := verifier.Verify(req)

			if tt.wantError == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantError)
			}
		})
	}
}

func TestNewStowrySignatureVerifier(t *testing.T) {
	store := keybackend.NewMapSecretStore(map[string]string{
		"test": "secret",
	})

	verifier := stowry.NewStowrySignatureVerifier(store)
	assert.NotNil(t, verifier)
}

func TestSignatureVerifier_Verify(t *testing.T) {
	const (
		accessKey = "TESTKEY"
		secretKey = "testsecret123"
	)

	store := keybackend.NewMapSecretStore(map[string]string{
		accessKey: secretKey,
	})

	verifier := stowry.NewSignatureVerifier("us-east-1", "s3", store)

	// Generate valid Stowry signature
	stowryTimestamp := time.Now().Unix()
	stowryExpires := int64(900)
	stowrySignature := stowrysign.Sign(secretKey, "GET", "/test.txt", stowryTimestamp, stowryExpires)

	// Generate valid AWS signature parameters (will fail signature check but tests delegation)
	awsTime := time.Now().UTC()
	awsDateStamp := awsTime.Format(stowry.DateFormat)
	awsAmzDate := awsTime.Format(stowry.DateTimeFormat)

	t.Run("delegates to StowrySignatureVerifier when X-Stowry-Signature present", func(t *testing.T) {
		query := url.Values{
			"X-Stowry-Credential": []string{accessKey},
			"X-Stowry-Date":       []string{fmt.Sprintf("%d", stowryTimestamp)},
			"X-Stowry-Expires":    []string{fmt.Sprintf("%d", stowryExpires)},
			"X-Stowry-Signature":  []string{stowrySignature},
		}
		req := &http.Request{
			Method: "GET",
			URL:    &url.URL{Path: "/test.txt", RawQuery: query.Encode()},
			Host:   "localhost:5708",
			Header: http.Header{},
		}

		err := verifier.Verify(req)
		assert.NoError(t, err)
	})

	t.Run("delegates to AWSSignatureVerifier when X-Amz-Signature present", func(t *testing.T) {
		query := url.Values{
			"X-Amz-Algorithm":     []string{"AWS4-HMAC-SHA256"},
			"X-Amz-Credential":    []string{fmt.Sprintf("%s/%s/us-east-1/s3/aws4_request", accessKey, awsDateStamp)},
			"X-Amz-Date":          []string{awsAmzDate},
			"X-Amz-Expires":       []string{"3600"},
			"X-Amz-SignedHeaders": []string{"host"},
			"X-Amz-Signature":     []string{"invalidsignature"},
		}
		req := &http.Request{
			Method: "GET",
			URL:    &url.URL{Path: "/test.txt", RawQuery: query.Encode()},
			Host:   "localhost:5708",
			Header: http.Header{},
		}

		err := verifier.Verify(req)
		// Should get signature mismatch (not "no supported signature"), proving delegation
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "signature mismatch")
	})

	t.Run("returns error when no signature present", func(t *testing.T) {
		query := url.Values{
			"some-other-param": []string{"value"},
		}
		req := &http.Request{
			Method: "GET",
			URL:    &url.URL{Path: "/test.txt", RawQuery: query.Encode()},
			Host:   "localhost:5708",
			Header: http.Header{},
		}

		err := verifier.Verify(req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no supported signature found")
	})

	t.Run("returns error for empty query", func(t *testing.T) {
		req := &http.Request{
			Method: "GET",
			URL:    &url.URL{Path: "/test.txt"},
			Host:   "localhost:5708",
			Header: http.Header{},
		}

		err := verifier.Verify(req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no supported signature found")
	})

	t.Run("prefers Stowry signature when both present", func(t *testing.T) {
		// When both signatures are present, Stowry should be checked first
		query := url.Values{
			// Stowry params (valid)
			"X-Stowry-Credential": []string{accessKey},
			"X-Stowry-Date":       []string{fmt.Sprintf("%d", stowryTimestamp)},
			"X-Stowry-Expires":    []string{fmt.Sprintf("%d", stowryExpires)},
			"X-Stowry-Signature":  []string{stowrySignature},
			// AWS params (would fail if checked)
			"X-Amz-Algorithm":     []string{"AWS4-HMAC-SHA256"},
			"X-Amz-Credential":    []string{fmt.Sprintf("%s/%s/us-east-1/s3/aws4_request", accessKey, awsDateStamp)},
			"X-Amz-Date":          []string{awsAmzDate},
			"X-Amz-Expires":       []string{"3600"},
			"X-Amz-SignedHeaders": []string{"host"},
			"X-Amz-Signature":     []string{"invalidsignature"},
		}
		req := &http.Request{
			Method: "GET",
			URL:    &url.URL{Path: "/test.txt", RawQuery: query.Encode()},
			Host:   "localhost:5708",
			Header: http.Header{},
		}

		// Should succeed because Stowry signature is valid and checked first
		err := verifier.Verify(req)
		assert.NoError(t, err)
	})
}
