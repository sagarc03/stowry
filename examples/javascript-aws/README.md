# JavaScript AWS SDK Example

Demonstrates using Stowry with [@aws-sdk/client-s3](https://www.npmjs.com/package/@aws-sdk/client-s3) and AWS Signature V4.

## Prerequisites

- Node.js 22+
- Stowry server running

## Run

```bash
# Start Stowry
stowry serve --config ../config.yaml --db-dsn /tmp/stowry.db --storage-path /tmp/data

# Run example
cd examples/javascript-aws
npm install
npm start
```

## What it demonstrates

- Configuring @aws-sdk/client-s3 for S3-compatible endpoints
- Using path-style addressing (`forcePathStyle: true`)
- Generating presigned URLs with `@aws-sdk/s3-request-presigner`
- Upload, download, and delete operations via presigned URLs

## Notes

- Stowry only supports query-string authentication (presigned URLs)
- The bucket name in the path is required but can be any value
