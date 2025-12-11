package stowry

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	SignatureAlgorithm = "AWS4-HMAC-SHA256"
	MaxExpiresSeconds  = 604800 // 7 days
	DateTimeFormat     = "20060102T150405Z"
	DateFormat         = "20060102"
)

// SignatureVerifier verifies AWS Signature V4 presigned URLs.
type SignatureVerifier struct {
	Region          string
	Service         string
	AccessKeyLookup func(accessKey string) (secretKey string, found bool)
}

// NewSignatureVerifier creates a new signature verifier.
//
// Parameters:
//   - region: AWS region (e.g., "us-east-1")
//   - service: AWS service name (e.g., "s3")
//   - lookup: Function to retrieve secret key by access key. Returns (secretKey, true) if found, ("", false) if not.
func NewSignatureVerifier(region, service string, lookup func(string) (string, bool)) *SignatureVerifier {
	return &SignatureVerifier{
		Region:          region,
		Service:         service,
		AccessKeyLookup: lookup,
	}
}

// Verify verifies an AWS Signature V4 presigned URL.
//
// This function implements AWS Signature Version 4 verification for presigned URLs,
// compatible with S3's authentication scheme. It validates all required query parameters,
// checks signature expiration, and verifies the HMAC-SHA256 signature.
//
// Required query parameters:
//   - X-Amz-Algorithm: Must be "AWS4-HMAC-SHA256"
//   - X-Amz-Credential: Format "access_key/date/region/service/aws4_request"
//   - X-Amz-Date: ISO8601 timestamp (YYYYMMDDTHHMMSSZ)
//   - X-Amz-Expires: Validity duration in seconds (1-604800)
//   - X-Amz-SignedHeaders: Semicolon-separated list of signed headers
//   - X-Amz-Signature: Hex-encoded HMAC-SHA256 signature
//
// The function performs the following validations:
//  1. Presence of all required parameters
//  2. Correct algorithm (AWS4-HMAC-SHA256)
//  3. Valid timestamp format
//  4. Expiration within allowed range (1 second to 7 days)
//  5. Request not expired (current time before timestamp + expires)
//  6. Credential format and component matching (date, region, service)
//  7. Access key exists (via lookup function)
//  8. Signature matches calculated signature
//
// Parameters:
//   - method: HTTP method (GET, PUT, DELETE, etc.)
//   - path: Request path
//   - query: URL query parameters including signature parameters
//   - headers: HTTP headers from the request (used for signed header verification)
//
// Returns an error if verification fails, nil if signature is valid.
//
// Example:
//
//	verifier := stowry.NewSignatureVerifier("us-east-1", "s3", lookupFunc)
//	err := verifier.Verify("GET", "/file.txt", r.URL.Query(), r.Header)
//	if err != nil {
//	    // Invalid signature
//	}
func (v *SignatureVerifier) Verify(method, path string, query url.Values, headers http.Header) error {
	params, err := v.extractParams(query)
	if err != nil {
		return err
	}

	if err := v.validateParams(params); err != nil {
		return err
	}

	secretKey, found := v.AccessKeyLookup(params.accessKey)
	if !found {
		return fmt.Errorf("invalid access key: %w", ErrUnauthorized)
	}

	expectedSignature := calculateSignature(
		secretKey,
		method,
		path,
		query,
		headers,
		params.requestTime,
		params.dateStamp,
		params.region,
		params.service,
		params.signedHeaders,
	)

	if !hmac.Equal([]byte(expectedSignature), []byte(params.signature)) {
		return fmt.Errorf("signature mismatch: %w", ErrUnauthorized)
	}

	return nil
}

type signatureParams struct {
	algorithm     string
	accessKey     string
	dateStamp     string
	region        string
	service       string
	requestTime   time.Time
	expires       int
	signedHeaders string
	signature     string
}

func (v *SignatureVerifier) extractParams(query url.Values) (*signatureParams, error) {
	amzAlgorithm := query.Get("X-Amz-Algorithm")
	amzCredential := query.Get("X-Amz-Credential")
	amzDate := query.Get("X-Amz-Date")
	amzExpires := query.Get("X-Amz-Expires")
	amzSignedHeaders := query.Get("X-Amz-SignedHeaders")
	amzSignature := query.Get("X-Amz-Signature")

	if amzAlgorithm == "" || amzCredential == "" || amzDate == "" ||
		amzExpires == "" || amzSignedHeaders == "" || amzSignature == "" {
		return nil, fmt.Errorf("missing required signature parameters: %w", ErrUnauthorized)
	}

	requestTime, err := time.Parse(DateTimeFormat, amzDate)
	if err != nil {
		return nil, fmt.Errorf("invalid X-Amz-Date format: %w", ErrUnauthorized)
	}

	expires := parseInt(amzExpires)
	if expires <= 0 || expires > MaxExpiresSeconds {
		return nil, fmt.Errorf("invalid X-Amz-Expires: must be between 1 and %d: %w", MaxExpiresSeconds, ErrUnauthorized)
	}

	credParts := strings.Split(amzCredential, "/")
	if len(credParts) != 5 {
		return nil, fmt.Errorf("invalid X-Amz-Credential format: %w", ErrUnauthorized)
	}

	if credParts[4] != "aws4_request" {
		return nil, fmt.Errorf("invalid credential terminator: expected aws4_request: %w", ErrUnauthorized)
	}

	return &signatureParams{
		algorithm:     amzAlgorithm,
		accessKey:     credParts[0],
		dateStamp:     credParts[1],
		region:        credParts[2],
		service:       credParts[3],
		requestTime:   requestTime,
		expires:       expires,
		signedHeaders: amzSignedHeaders,
		signature:     amzSignature,
	}, nil
}

func (v *SignatureVerifier) validateParams(params *signatureParams) error {
	if params.algorithm != SignatureAlgorithm {
		return fmt.Errorf("invalid algorithm: expected %s, got %s: %w", SignatureAlgorithm, params.algorithm, ErrUnauthorized)
	}

	if time.Now().After(params.requestTime.Add(time.Duration(params.expires) * time.Second)) {
		return fmt.Errorf("signature expired: %w", ErrUnauthorized)
	}

	expectedDate := params.requestTime.Format(DateFormat)
	if params.dateStamp != expectedDate {
		return fmt.Errorf("credential date mismatch: %w", ErrUnauthorized)
	}

	if params.region != v.Region {
		return fmt.Errorf("region mismatch: expected %s, got %s: %w", v.Region, params.region, ErrUnauthorized)
	}

	if params.service != v.Service {
		return fmt.Errorf("service mismatch: expected %s, got %s: %w", v.Service, params.service, ErrUnauthorized)
	}

	return nil
}

func calculateSignature(
	secretKey, method, path string,
	query url.Values,
	headers http.Header,
	requestTime time.Time,
	dateStamp, region, service, signedHeaders string,
) string {
	canonicalRequest := buildCanonicalRequest(method, path, query, headers, signedHeaders)

	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request", dateStamp, region, service)
	stringToSign := buildStringToSign(requestTime, credentialScope, canonicalRequest)

	signingKey := deriveSigningKey(secretKey, dateStamp, region, service)

	signature := hmacSHA256(signingKey, []byte(stringToSign))
	return hex.EncodeToString(signature)
}

func buildCanonicalRequest(method, path string, query url.Values, headers http.Header, signedHeaders string) string {
	canonicalQuery := buildCanonicalQueryString(query)
	canonicalHeaders := buildCanonicalHeaders(headers, signedHeaders)
	payloadHash := "UNSIGNED-PAYLOAD"

	return fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		method,
		path,
		canonicalQuery,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	)
}

// buildCanonicalHeaders builds the canonical headers string from the signed headers list.
// Headers are sorted alphabetically and formatted as "name:value\n".
func buildCanonicalHeaders(headers http.Header, signedHeaders string) string {
	headerNames := strings.Split(signedHeaders, ";")
	sort.Strings(headerNames)

	var result strings.Builder
	for _, name := range headerNames {
		// Header names in signedHeaders are lowercase
		value := headers.Get(name)
		// Trim whitespace and collapse multiple spaces
		value = strings.TrimSpace(value)
		result.WriteString(name)
		result.WriteString(":")
		result.WriteString(value)
		result.WriteString("\n")
	}
	return result.String()
}

func buildCanonicalQueryString(query url.Values) string {
	params := url.Values{}
	for k, v := range query {
		if k != "X-Amz-Signature" {
			params[k] = v
		}
	}
	return params.Encode()
}

func buildStringToSign(requestTime time.Time, credentialScope, canonicalRequest string) string {
	hashedCanonicalRequest := sha256Hash(canonicalRequest)
	return fmt.Sprintf("%s\n%s\n%s\n%s",
		SignatureAlgorithm,
		requestTime.Format(DateTimeFormat),
		credentialScope,
		hashedCanonicalRequest,
	)
}

func deriveSigningKey(secretKey, dateStamp, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secretKey), []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))
	return kSigning
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func sha256Hash(data string) string {
	h := sha256.New()
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

func parseInt(s string) int {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	if err != nil {
		return 0
	}
	return n
}
