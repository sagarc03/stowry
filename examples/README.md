# Stowry SDK Examples

Examples demonstrating how to use Stowry with AWS S3 SDKs to generate presigned URLs.

## Prerequisites

Start Stowry with the example config:

```bash
# From repository root
stowry serve --config examples/config.yaml --db-dsn /tmp/stowry.db --storage-path /tmp/data
```

Or with Docker:

```bash
docker run --rm -p 5708:5708 \
  -v $(pwd)/examples/config.yaml:/config.yaml:ro \
  ghcr.io/sagarc03/stowry:latest serve --config /config.yaml --db-dsn :memory:
```

## Run the Examples

### Go

Requires Go 1.25+

```bash
cd examples/go
go mod tidy
go run main.go
```

### Python

Requires Python 3.11+

```bash
cd examples/python
python -m venv .venv
source .venv/bin/activate  # Windows: .venv\Scripts\activate
pip install -r requirements.txt
python main.py
```

### JavaScript

Requires Node.js 22+ (LTS)

```bash
cd examples/javascript
npm install
npm start
```

## Expected Output

All examples produce similar output:

```text
=== Upload ===
Uploaded: example/hello.txt

=== Download ===
Content: Hello from <SDK>!

=== Presigned URLs ===
GET URL: http://localhost:5708/example/hello.txt?X-Amz-Algorithm=...
PUT URL: http://localhost:5708/example/presigned-upload.txt?X-Amz-Algorithm=...
DELETE URL: http://localhost:5708/example/hello.txt?X-Amz-Algorithm=...

=== Delete ===
Deleted: example/hello.txt
```

## How It Works

Stowry supports **presigned URL authentication** (AWS Signature V4 query parameters). The examples:

1. Use the AWS SDK to generate presigned URLs
2. Make HTTP requests with the presigned URLs using standard HTTP clients

### Configuration

All examples read credentials from `../config.yaml`:

```yaml
auth:
  region: us-east-1
  service: s3
  keys:
    - access_key: <your-access-key>
      secret_key: <your-secret-key>
```

Generate your own keys:

```bash
# Access key (20 chars)
openssl rand -hex 10 | tr '[:lower:]' '[:upper:]'

# Secret key (40 chars)
openssl rand -hex 20
```

## Bucket Name Handling

Stowry doesn't have real buckets - the bucket name becomes the first directory in the file path.

```text
Bucket: "example", Key: "hello.txt" → stored at: example/hello.txt
```

All examples use `Bucket: "example"` which acts as a namespace/directory.

### Request Methods

| SDK | Presign | HTTP Client |
|-----|---------|-------------|
| Go | SDK presigner | SDK client (works with public access) |
| Python | boto3 `generate_presigned_url` | `requests` library |
| JavaScript | `@aws-sdk/s3-request-presigner` | Native `fetch` |

## Supported Operations

Stowry supports these operations via presigned URLs:

- **PUT** - Upload files
- **GET** - Download files
- **DELETE** - Delete files

**Not supported:** ListObjects, HeadObject, multipart uploads, and other S3 operations.

## Notes

- Path-style addressing must be enabled (`UsePathStyle: true` / `forcePathStyle: true`)
- **Signature V4 required** - Stowry only supports AWS Signature V4 (`signature_version='s3v4'` in boto3)
- Presigned URLs include the signature in query parameters
- URLs expire after the configured time (default: 15 minutes)
