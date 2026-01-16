# Stowry SDK Examples

Examples demonstrating how to use Stowry with presigned URLs.

## SDKs

Stowry provides lightweight native SDKs for presigned URL generation:

| Language | Package | Install |
|----------|---------|---------|
| Go | [stowry-go](https://github.com/sagarc03/stowry-go) | `go get github.com/sagarc03/stowry-go` |
| Python | [stowrypy](https://pypi.org/project/stowrypy/) | `pip install stowrypy` |
| JavaScript | [stowryjs](https://www.npmjs.com/package/stowryjs) | `npm install stowryjs` |

Stowry also supports AWS Signature V4, so you can use official AWS SDKs (boto3, aws-sdk-go-v2, @aws-sdk/client-s3).

## Signing Schemes

- **Stowry native signing** - Simple, lightweight (`stowry/` folder)
- **AWS Signature V4** - Compatible with AWS SDKs (`aws/` folder)

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

## Stowry Native Signing

Uses the lightweight Stowry SDKs with native signing scheme.

### Go (stowry-go)

```bash
cd examples/stowry/go
go mod tidy
go run main.go
```

### Python (stowrypy)

```bash
cd examples/stowry/python
python -m venv .venv
source .venv/bin/activate  # Windows: .venv\Scripts\activate
pip install -r requirements.txt
python main.py
```

### JavaScript (stowryjs)

```bash
cd examples/stowry/javascript
npm install
npm start
```

## AWS Signature V4

Uses official AWS SDKs with Signature V4 authentication.

### Go (aws-sdk-go-v2)

```bash
cd examples/aws/go
go mod tidy
go run main.go
```

### Python (boto3)

```bash
cd examples/aws/python
python -m venv .venv
source .venv/bin/activate  # Windows: .venv\Scripts\activate
pip install -r requirements.txt
python main.py
```

### JavaScript (@aws-sdk/client-s3)

```bash
cd examples/aws/javascript
npm install
npm start
```

## Expected Output

All examples produce similar output:

```text
=== Upload ===
Uploaded: /hello.txt

=== Download ===
Content: Hello from <SDK>!

=== Presigned URLs ===
GET URL: http://localhost:5708/...
PUT URL: http://localhost:5708/...
DELETE URL: http://localhost:5708/...

=== Delete ===
Deleted: /hello.txt
```

## Configuration

All examples read credentials from `config.yaml`:

```yaml
auth:
  region: us-east-1
  service: s3
  # WARNING: Example keys only - do not use in production
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

## URL Formats

**Stowry native signing:**
```
http://localhost:5708/path?X-Stowry-Credential=...&X-Stowry-Date=...&X-Stowry-Expires=...&X-Stowry-Signature=...
```

**AWS Signature V4:**
```
http://localhost:5708/bucket/key?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=...&X-Amz-Date=...&X-Amz-Expires=...&X-Amz-SignedHeaders=...&X-Amz-Signature=...
```

## Notes

- Stowry native signing doesn't require bucket prefix in path
- AWS SDK examples require path-style addressing (`UsePathStyle: true`)
- AWS SDK for Python (boto3) requires `signature_version='s3v4'`
