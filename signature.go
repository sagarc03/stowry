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

	stowrysign "github.com/sagarc03/stowry-go"
)

const (
	SignatureAlgorithm = "AWS4-HMAC-SHA256"
	MaxExpiresSeconds  = 604800 // 7 days
	DateTimeFormat     = "20060102T150405Z"
	DateFormat         = "20060102"

	// AWS Signature V4 query parameter names
	AWSAlgorithmParam     = "X-Amz-Algorithm"
	AWSCredentialParam    = "X-Amz-Credential" //nolint:gosec // This is a param name, not a credential
	AWSDateParam          = "X-Amz-Date"
	AWSExpiresParam       = "X-Amz-Expires"
	AWSSignedHeadersParam = "X-Amz-SignedHeaders"
	AWSSignatureParam     = "X-Amz-Signature"
)

// SignatureVerifier verifies signed requests using either AWS Signature V4 or native Stowry signing.
// It automatically detects the signing scheme from the request query parameters.
type SignatureVerifier struct {
	stowryVerifier *StowrySignatureVerifier
	awsVerifier    *AWSSignatureVerifier
}

// NewSignatureVerifier creates a new unified signature verifier that supports both
// AWS Signature V4 and native Stowry signing schemes.
//
// Parameters:
//   - region: AWS region (e.g., "us-east-1")
//   - service: AWS service name (e.g., "s3")
//   - lookup: Function to retrieve secret key by access key. Returns (secretKey, true) if found, ("", false) if not.
func NewSignatureVerifier(region, service string, lookup func(string) (string, bool)) *SignatureVerifier {
	return &SignatureVerifier{
		stowryVerifier: NewStowrySignatureVerifier(lookup),
		awsVerifier:    NewAWSSignatureVerifier(region, service, lookup),
	}
}

// Verify verifies a signed HTTP request using the appropriate signing scheme.
// It detects the scheme from query parameters:
//   - X-Stowry-Signature: Uses native Stowry verification
//   - X-Amz-Signature: Uses AWS Signature V4 verification
//
// Returns ErrUnauthorized if no valid signature is present or verification fails.
func (v *SignatureVerifier) Verify(r *http.Request) error {
	query := r.URL.Query()

	if _, ok := query[stowrysign.StowrySignatureParam]; ok {
		return v.stowryVerifier.Verify(r)
	}

	if _, ok := query[AWSSignatureParam]; ok {
		return v.awsVerifier.Verify(r)
	}

	return ErrUnauthorized
}

// StowrySignatureVerifier verifies Stowry native presigned URLs.
type StowrySignatureVerifier struct {
	AccessKeyLookup func(accessKey string) (secretKey string, found bool)
}

// NewStowrySignatureVerifier creates a new Stowry signature verifier.
//
// Parameters:
//   - lookup: Function to retrieve secret key by access key. Returns (secretKey, true) if found, ("", false) if not.
func NewStowrySignatureVerifier(lookup func(string) (string, bool)) *StowrySignatureVerifier {
	return &StowrySignatureVerifier{
		AccessKeyLookup: lookup,
	}
}

// Verify verifies a Stowry native presigned URL from an HTTP request.
// Returns an error if verification fails, nil if signature is valid.
func (v *StowrySignatureVerifier) Verify(r *http.Request) error {
	query := r.URL.Query()

	credential := query.Get(stowrysign.StowryCredentialParam)
	dateStr := query.Get(stowrysign.StowryDateParam)
	expiresStr := query.Get(stowrysign.StowryExpiresParam)
	signature := query.Get(stowrysign.StowrySignatureParam)

	if credential == "" || dateStr == "" || expiresStr == "" || signature == "" {
		return fmt.Errorf("missing required signature parameters: %w", ErrUnauthorized)
	}

	timestamp := parseInt64(dateStr)
	expires := parseInt64(expiresStr)

	if expires <= 0 || expires > stowrysign.MaxExpires {
		return fmt.Errorf("invalid expires: must be between 1 and %d: %w", stowrysign.MaxExpires, ErrUnauthorized)
	}

	if time.Now().Unix() > timestamp+expires {
		return fmt.Errorf("signature expired: %w", ErrUnauthorized)
	}

	secretKey, found := v.AccessKeyLookup(credential)
	if !found {
		return fmt.Errorf("invalid credential: %w", ErrUnauthorized)
	}

	expectedSignature := stowrysign.Sign(secretKey, r.Method, r.URL.Path, timestamp, expires)

	if !hmac.Equal([]byte(expectedSignature), []byte(signature)) {
		return fmt.Errorf("signature mismatch: %w", ErrUnauthorized)
	}

	return nil
}

// AWSSignatureVerifier verifies AWS Signature V4 presigned URLs.
type AWSSignatureVerifier struct {
	Region          string
	Service         string
	AccessKeyLookup func(accessKey string) (secretKey string, found bool)
}

// NewAWSSignatureVerifier creates a new AWS signature verifier.
//
// Parameters:
//   - region: AWS region (e.g., "us-east-1")
//   - service: AWS service name (e.g., "s3")
//   - lookup: Function to retrieve secret key by access key. Returns (secretKey, true) if found, ("", false) if not.
func NewAWSSignatureVerifier(region, service string, lookup func(string) (string, bool)) *AWSSignatureVerifier {
	return &AWSSignatureVerifier{
		Region:          region,
		Service:         service,
		AccessKeyLookup: lookup,
	}
}

// Verify verifies an AWS Signature V4 presigned URL from an HTTP request.
// Returns an error if verification fails, nil if signature is valid.
func (v *AWSSignatureVerifier) Verify(r *http.Request) error {
	query := r.URL.Query()
	headers := r.Header.Clone()
	headers.Set("Host", r.Host)

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
		r.Method,
		r.URL.Path,
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
	expires       int64
	signedHeaders string
	signature     string
}

func (v *AWSSignatureVerifier) extractParams(query url.Values) (*signatureParams, error) {
	amzAlgorithm := query.Get(AWSAlgorithmParam)
	amzCredential := query.Get(AWSCredentialParam)
	amzDate := query.Get(AWSDateParam)
	amzExpires := query.Get(AWSExpiresParam)
	amzSignedHeaders := query.Get(AWSSignedHeadersParam)
	amzSignature := query.Get(AWSSignatureParam)

	if amzAlgorithm == "" || amzCredential == "" || amzDate == "" ||
		amzExpires == "" || amzSignedHeaders == "" || amzSignature == "" {
		return nil, fmt.Errorf("missing required signature parameters: %w", ErrUnauthorized)
	}

	requestTime, err := time.Parse(DateTimeFormat, amzDate)
	if err != nil {
		return nil, fmt.Errorf("invalid X-Amz-Date format: %w", ErrUnauthorized)
	}

	expires := parseInt64(amzExpires)
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

func (v *AWSSignatureVerifier) validateParams(params *signatureParams) error {
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
		if k != AWSSignatureParam {
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

func parseInt64(s string) int64 {
	var n int64
	_, err := fmt.Sscanf(s, "%d", &n)
	if err != nil {
		return 0
	}
	return n
}
