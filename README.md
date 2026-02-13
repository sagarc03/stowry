# Stowry

[![CI](https://github.com/sagarc03/stowry/actions/workflows/ci.yaml/badge.svg)](https://github.com/sagarc03/stowry/actions/workflows/ci.yaml)
[![Go Report Card](https://goreportcard.com/badge/github.com/sagarc03/stowry)](https://goreportcard.com/report/github.com/sagarc03/stowry)

A lightweight, self-hosted object storage server with AWS Signature V4 authentication.

**Use cases**: Local development, self-hosting, static site hosting, SPA deployment, simple file storage.

## Features

- **AWS Sig V4 authentication** - Uses AWS Signature V4 presigned URLs (not S3-compatible API)
- **Three server modes** - Object storage API, static file server, or SPA host
- **Minimal dependencies** - Single binary, SQLite (3.24+) or PostgreSQL for metadata
- **Soft deletion** - Files are recoverable until cleanup runs
- **Atomic writes** - No partial or corrupted files
- **Pluggable storage** - Filesystem now, S3/GCS ready interface

## Quick Start

```bash
# Using Docker
docker run -p 5708:5708 -v ./data:/data ghcr.io/sagarc03/stowry:latest

# Using binary
./stowry serve
```

Server starts at `http://localhost:5708`

## Client SDKs

Generate presigned URLs to interact with Stowry:

| Language   | Package                                              | Install                                 |
|------------|------------------------------------------------------|-----------------------------------------|
| Go         | [stowry-go](https://github.com/sagarc03/stowry-go)   | `go get github.com/sagarc03/stowry-go`  |
| Python     | [stowrypy](https://pypi.org/project/stowrypy/)       | `pip install stowrypy`                  |
| JavaScript | [stowryjs](https://www.npmjs.com/package/stowryjs)   | `npm install stowryjs`                  |

AWS SDKs (boto3, aws-sdk-go-v2, @aws-sdk/client-s3) also work for generating presigned URLs.

See [examples](examples) for usage.

## Client CLI

For command-line access without writing code:

```bash
# Install
go install github.com/sagarc03/stowry/cmd/stowry-cli@latest

# Configure
export STOWRY_SERVER=http://localhost:5708
export STOWRY_ACCESS_KEY=your-access-key
export STOWRY_SECRET_KEY=your-secret-key

# Upload (uses local path as remote path)
stowry-cli upload ./images/photo.jpg

# Upload with explicit remote path
stowry-cli upload ./file.txt custom/path.txt

# Download
stowry-cli download images/photo.jpg

# List (store mode only)
stowry-cli list --prefix images/

# Delete
stowry-cli delete images/photo.jpg
```

See [Client CLI Reference](https://stowry.dev/client-cli) for full documentation.

## Installation

### Docker

```bash
docker pull ghcr.io/sagarc03/stowry:latest

# With persistent storage
docker run -d \
  --name stowry \
  -p 5708:5708 \
  -v ./data:/data \
  -v ./stowry.db:/stowry.db \
  ghcr.io/sagarc03/stowry:latest
```

### Binary

Download from [Releases](https://github.com/sagarc03/stowry/releases):

```bash
# Linux
curl -LO https://github.com/sagarc03/stowry/releases/latest/download/stowry_linux_amd64.tar.gz
tar xzf stowry_linux_amd64.tar.gz
./stowry serve

# macOS
curl -LO https://github.com/sagarc03/stowry/releases/latest/download/stowry_darwin_arm64.tar.gz
tar xzf stowry_darwin_arm64.tar.gz
./stowry serve
```

### From Source

```bash
go install github.com/sagarc03/stowry/cmd/stowry@latest
```

## CLI Commands

```bash
# Start the server
stowry serve [--port 5708] [--mode store|static|spa]

# Import files into storage
stowry add [--dest prefix/] [--recursive] <file1> [file2] ...

# Remove files from storage (soft-delete)
stowry remove [--prefix] <path1> [path2] ...

# Initialize metadata from existing files
stowry init [--storage ./data]

# Clean up soft-deleted files
stowry cleanup [--limit 100]
```

### Global Flags

| Flag        | Env Var                | Default       | Description         |
|-------------|------------------------|---------------|---------------------|
| `--config`  | -                      | `config.yaml` | Config file path    |
| `--db-type` | `STOWRY_DATABASE_TYPE` | `sqlite`      | Database type       |
| `--db-dsn`  | `STOWRY_DATABASE_DSN`  | `stowry.db`   | Database connection |
| `--storage` | `STOWRY_STORAGE_PATH`  | `./data`      | Storage directory   |

## Configuration

Create `config.yaml`:

```yaml
server:
  port: 5708
  mode: store  # store | static | spa
  max_upload_size: 0  # Maximum upload size in bytes (0 = unlimited)

service:
  cleanup_timeout: 30  # Cleanup operation timeout in seconds

database:
  type: sqlite      # sqlite | postgres
  dsn: stowry.db    # file path or connection string
  tables:
    meta_data: stowry_metadata

storage:
  path: ./data

auth:
  read: public   # public | private
  write: public  # public | private
  aws:
    region: us-east-1
    service: s3
  keys:
    inline:
      - access_key: YOUR_ACCESS_KEY
        secret_key: YOUR_SECRET_KEY
    # file: /path/to/keys.json

log:
  level: info  # debug | info | warn | error
```

Environment variables use `STOWRY_` prefix: `STOWRY_SERVER_PORT=8080`

## API

### Upload

```bash
curl -X PUT http://localhost:5708/path/to/file.txt \
  -H "Content-Type: text/plain" \
  -d "Hello, World!"
```

### Download

```bash
curl http://localhost:5708/path/to/file.txt
```

### Head (Metadata Only)

```bash
curl -I http://localhost:5708/path/to/file.txt
```

Returns `Content-Type`, `Content-Length`, `ETag`, and `Last-Modified` headers without the file body. Supports `If-None-Match` and `If-Modified-Since` conditional headers.

### Delete

```bash
curl -X DELETE http://localhost:5708/path/to/file.txt
```

### List Objects

```bash
curl "http://localhost:5708/?prefix=path/&limit=100"
```

Response:

```json
{
  "items": [
    {
      "path": "path/to/file.txt",
      "content_type": "text/plain",
      "etag": "abc123...",
      "file_size_bytes": 13,
      "created_at": "2024-01-15T10:00:00Z",
      "updated_at": "2024-01-15T10:00:00Z"
    }
  ],
  "next_cursor": "..."
}
```

### Authentication

When `auth.read` or `auth.write` is set to `private`, requests require AWS Signature V4 presigned URL parameters.

#### Generating Keys

Access keys and secret keys are arbitrary strings. Generate them with:

```bash
# Access key (20 chars)
openssl rand -hex 10 | tr '[:lower:]' '[:upper:]'

# Secret key (40 chars)
openssl rand -hex 20
```

Or use any password generator.

#### Presigned URL Format

```text
?X-Amz-Algorithm=AWS4-HMAC-SHA256
&X-Amz-Credential=ACCESS_KEY/20240115/us-east-1/s3/aws4_request
&X-Amz-Date=20240115T100000Z
&X-Amz-Expires=3600
&X-Amz-SignedHeaders=host
&X-Amz-Signature=...
```

You can use S3 SDKs to generate presigned URL signatures, but note that Stowry's API is not S3-compatible.

## Server Modes

### Store (default)

Object storage API. Returns 404 for missing paths.

### Static

Static file server. Serves `index.html` for directory paths:

- `/docs` → `/docs/index.html`

### SPA

Single Page Application mode. Returns `/index.html` for all 404s, enabling client-side routing.

## Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: stowry
spec:
  selector:
    matchLabels:
      app: stowry
  template:
    metadata:
      labels:
        app: stowry
    spec:
      securityContext:
        runAsUser: 65532
        runAsGroup: 65532
        fsGroup: 65532
      containers:
        - name: stowry
          image: ghcr.io/sagarc03/stowry:latest
          ports:
            - containerPort: 5708
          volumeMounts:
            - name: data
              mountPath: /data
      volumes:
        - name: data
          persistentVolumeClaim:
            claimName: stowry-data
```

## Development

This project uses [Task](https://taskfile.dev/) as a task runner.

```bash
# Install Task (macOS)
brew install go-task

# List available tasks
task --list

# Run tests
task test

# Run linter
task lint

# Build binary
task build

# Run all checks (fmt, lint, test)
task check

# Run examples
task examples:stowry   # Start server
task examples:go-aws   # Run Go AWS example
```

## Contributing

Contributions are welcome! Please follow these steps:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Run tests and linter (`task check`)
5. Commit your changes (`git commit -m 'Add amazing feature'`)
6. Push to the branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

### Guidelines

- Follow existing code style
- Add tests for new features
- Update documentation as needed
- Keep commits focused and atomic

## License

MIT
