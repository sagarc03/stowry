# Go Native Signing Example

Demonstrates using Stowry with the `github.com/sagarc03/stowry/sign` package and native signing.

## Prerequisites

- Go 1.21+
- Stowry server running

## Run

```bash
# Start Stowry
stowry migrate --config ../config.yaml --db-dsn /tmp/stowry.db --storage-path /tmp/data
stowry serve   --config ../config.yaml --db-dsn /tmp/stowry.db --storage-path /tmp/data

# Run example
cd examples/go-native
go mod tidy
go run main.go
```

## What it demonstrates

- Creating a signing client
- Uploading files with presigned PUT URLs
- Downloading files with presigned GET URLs
- Generating presigned URLs for GET, PUT, DELETE
- Deleting files with presigned DELETE URLs
