# Stowry Examples

Examples demonstrating how to use Stowry with presigned URLs.

## SDK Examples

Simple examples showing how to use Stowry SDKs for upload, download, and delete operations.

| Example | SDK | Signing |
|---------|-----|---------|
| [go-native](./go-native/) | [stowry-go](https://github.com/sagarc03/stowry-go) | Native |
| [go-aws](./go-aws/) | [aws-sdk-go-v2](https://github.com/aws/aws-sdk-go-v2) | AWS Sig V4 |
| [python-native](./python-native/) | [stowrypy](https://pypi.org/project/stowrypy/) | Native |
| [python-aws](./python-aws/) | [boto3](https://boto3.amazonaws.com/) | AWS Sig V4 |
| [javascript-native](./javascript-native/) | [stowryjs](https://www.npmjs.com/package/stowryjs) | Native |
| [javascript-aws](./javascript-aws/) | [@aws-sdk/client-s3](https://www.npmjs.com/package/@aws-sdk/client-s3) | AWS Sig V4 |

## Application Examples

Full-stack applications demonstrating real-world usage patterns.

| Example | Stack | Description |
|---------|-------|-------------|
| [app-nextjs](./app-nextjs/) | Next.js + Better Upload | Server components, direct browser uploads |
| [app-flask-react](./app-flask-react/) | Flask + React SPA | Traditional backend/frontend separation |

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

## Configuration

All examples read credentials from `config.yaml`:

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

## Signing Schemes

**Stowry native signing** - Lightweight, simpler URL format:
```
http://localhost:5708/path?X-Stowry-Credential=...&X-Stowry-Date=...&X-Stowry-Expires=...&X-Stowry-Signature=...
```

**AWS Signature V4** - Compatible with AWS SDKs:
```
http://localhost:5708/bucket/key?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=...&X-Amz-Date=...&X-Amz-Expires=...&X-Amz-SignedHeaders=...&X-Amz-Signature=...
```

## Notes

- Stowry native signing doesn't require bucket prefix in path
- AWS SDK examples require path-style addressing
- AWS SDK for Python (boto3) requires `signature_version='s3v4'`
