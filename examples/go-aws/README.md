# Go AWS SDK Example

Demonstrates using Stowry with [aws-sdk-go-v2](https://github.com/aws/aws-sdk-go-v2) and AWS Signature V4.

## Prerequisites

- Go 1.21+
- Stowry server running

## Run

```bash
# Start Stowry
stowry serve --config ../config.yaml --db-dsn /tmp/stowry.db --storage-path /tmp/data

# Run example
cd examples/go-aws
go mod tidy
go run main.go
```

## What it demonstrates

- Configuring aws-sdk-go-v2 for S3-compatible endpoints
- Using path-style addressing (`UsePathStyle: true`)
- Generating presigned URLs with `s3.PresignClient`
- Upload, download, and delete operations via presigned URLs

## Notes

- Stowry only supports query-string authentication (presigned URLs)
- The bucket name in the path is required but can be any value
