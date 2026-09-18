# Stowry

[![CI](https://github.com/sagarc03/stowry/actions/workflows/ci.yaml/badge.svg)](https://github.com/sagarc03/stowry/actions/workflows/ci.yaml)

A lightweight, self-hosted object storage server with AWS Signature V4 authentication.

**Use cases**: Local development, self-hosting, static site hosting, SPA deployment, simple file storage.

## Features

- **AWS Sig V4 authentication** - Uses AWS Signature V4 presigned URLs (not S3-compatible API)
- **Three server modes** - Object storage API, static file server, or SPA host
- **Minimal dependencies** - Single binary, SQLite (3.24+) or PostgreSQL for metadata
- **Server and client in one binary** - Serve, or upload and download against a server
- **Atomic writes** - No partial or corrupted files
- **Pluggable storage** - Filesystem now, S3/GCS ready interface

## Quick Start

```bash
# Using Docker
docker run -p 5708:5708 -v ./data:/data ghcr.io/sagarc03/stowry:latest

# Using binary, keeping nothing (both stores default to memory)
./stowry serve

# Using binary, keeping everything
./stowry migrate --db-dsn stowry.db --storage-path ./data
./stowry serve   --db-dsn stowry.db --storage-path ./data
```

Server starts at `http://localhost:5708`.

With no configuration both the metadata and the objects are held in memory and
are gone when the process exits. A database on disk is created by `migrate`:
`serve` never migrates one, so that pointing it at the wrong path fails instead
of silently starting an empty store.

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

The same binary is the client. It signs with the key pair the server verifies:

```bash
export STOWRY_ENDPOINT=http://localhost:5708
export STOWRY_ACCESS_KEY=your-access-key
export STOWRY_SECRET_KEY=your-secret-key

# Upload (remote path defaults to the local path)
stowry upload ./images/photo.jpg

# Upload with an explicit remote path
stowry upload ./file.txt custom/path.txt

# Upload a directory: its contents go under the remote path
stowry upload ./site/ assets

# Download ("-" writes to stdout)
stowry download images/photo.jpg

# List, oldest first
stowry list images/ --all

# Delete
stowry delete images/photo.jpg
```

Upload, delete and list need the server in `store` mode.

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

#### Image variants

Every variant ships the same binary at `/stowry`, runs as uid/gid `65532`, stores
data in `/data` and is published for `linux/amd64` and `linux/arm64`.

| Tag | Base | Notes |
| --- | --- | --- |
| `latest`, `v0.3.0` | `scratch` | Default. Smallest image, nothing but the server and a CA bundle. |
| `distroless`, `v0.3.0-distroless` | `gcr.io/distroless/static-debian13` | Debian filesystem layout, tzdata and a maintained CA bundle, still no shell. |
| `debian`, `v0.3.0-debian` | `debian:trixie-slim` | glibc and a full Debian userland, for Debian-based tooling and scanners. |
| `alpine`, `v0.3.0-alpine` | `alpine:3.21` | Busybox shell and `apk`, for debugging inside a running container. |

Variant tags follow the same scheme as the default image, so `v0`, `v0.3` and
`v0.3.0` each have `-alpine`, `-debian` and `-distroless` counterparts.

```bash
docker pull ghcr.io/sagarc03/stowry:alpine
docker pull ghcr.io/sagarc03/stowry:v0.3-distroless
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
go install github.com/sagarc03/stowry@latest
```

## CLI Commands

Server:

```bash
stowry serve      # start the HTTP server
stowry migrate    # create the metadata schema
stowry validate   # check the schema without changing it
stowry populate   # record files already in the storage directory
```

Client:

```bash
stowry upload <local> [remote]     # a file, or a directory whole
stowry download <remote> [local]   # "-" writes to stdout
stowry list [prefix]               # --limit, --cursor, --all
stowry delete <remote>...          # every path attempted
```

`populate` is how a directory of existing files gets served without uploading
anything: it reads the files and records where they already are.

### Global Flags

Every setting is reachable three ways, highest first: a flag, an environment
variable, then the config files. `stowry <command> --help` lists each flag with
the variable that reaches the same setting.

| Flag             | Env Var                | Default          | Description           |
|------------------|------------------------|------------------|-----------------------|
| `--config`       | `STOWRY_CONFIG`        | `./config.yaml`  | Config files, merged left to right |
| `--db-type`      | `STOWRY_DATABASE_TYPE` | `sqlite`         | Database type         |
| `--db-dsn`       | `STOWRY_DATABASE_DSN`  | `:memory:`       | Database connection   |
| `--storage-path` | `STOWRY_STORAGE_PATH`  | `:memory:`       | Storage directory     |
| `--endpoint`     | `STOWRY_ENDPOINT`      | `http://localhost:5708` | Server the client commands talk to |
| `--access-key`   | `STOWRY_ACCESS_KEY`    | -                | Access key for signed requests |
| `--secret-key`   | `STOWRY_SECRET_KEY`    | -                | Secret key for signed requests |

## Configuration

Create `config.yaml`:

```yaml
server:
  port: 5708
  mode: store  # store | static | spa
  max_upload_size: 0  # Maximum upload size in bytes (0 = unlimited)
  error_document: ""  # Custom 404 page path for static mode (default: built-in HTML)

service:
  cleanup_timeout: 30  # Cleanup operation timeout in seconds

database:
  type: sqlite      # sqlite | postgres
  dsn: stowry.db    # file path, connection string, or :memory:
  tables:
    meta_data: stowry_metadata

storage:
  path: ./data      # directory, or :memory:

# The server the client commands talk to.
endpoint: http://localhost:5708

auth:
  read: public   # public | private
  write: public  # public | private
  aws:
    region: us-east-1
    service: s3
  access_key: YOUR_ACCESS_KEY
  secret_key: YOUR_SECRET_KEY
  keys:
    # file: /path/to/keys.json  # several pairs

log:
  level: info  # debug | info | warn | error
```

> **Note:** `static` and `spa` modes serve browsers, which cannot sign requests, so all access is public in those modes. Setting `auth.read: private` together with `mode: static` or `mode: spa` is a startup error rather than a silently public site — use `store` mode if you need signed reads. `auth.write` is ignored there (writes are not served at all) and only logs a warning. The `max_upload_size` setting only applies in `store` mode.

Environment variables use the `STOWRY_` prefix, with `.` becoming `_`:
`server.port` is `STOWRY_SERVER_PORT`. The credentials are the one exception,
dropping the section name:

```sh
export STOWRY_ACCESS_KEY=YOUR_ACCESS_KEY
export STOWRY_SECRET_KEY=YOUR_SECRET_KEY
```

Each setting answers to exactly one variable, so `STOWRY_AUTH_ACCESS_KEY` is
not read. The same pair is also available on `serve` as `-a/--access-key` and
`-k/--secret-key`, though a flag value is visible in the process list.

A deployment with more than one key pair sets `auth.keys.file`
(`STOWRY_AUTH_KEYS_FILE`) instead; the two are merged, and the file wins on a
repeated access key.

## API

> **Note:** Upload (PUT) and Delete are only available in `store` mode. Static and SPA modes return `405 Method Not Allowed`.

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

Object storage API with full CRUD. Returns 404 for missing paths.

### Static

Read-only static file server with S3+CloudFront-style path resolution (public access):

- `/about` → `about` (exact) → `about.html` → `about/index.html`
- `/docs/` → `docs/index.html`
- `/` → `index.html`
- Missing paths return an HTML 404 page (configurable via `error_document`)

Put the files in the storage directory and run `stowry populate`; static and spa modes route no writes.

### SPA

Read-only Single Page Application host (public access). Returns `/index.html` for all 404s, enabling client-side routing.

Put the files in the storage directory and run `stowry populate`; static and spa modes route no writes.

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
