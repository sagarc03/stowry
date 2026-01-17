# JavaScript Native Signing Example

Demonstrates using Stowry with [stowryjs](https://www.npmjs.com/package/stowryjs) SDK and native signing.

## Prerequisites

- Node.js 18+
- Stowry server running

## Run

```bash
# Start Stowry
stowry serve --config ../config.yaml --db-dsn /tmp/stowry.db --storage-path /tmp/data

# Run example
cd examples/javascript-native
npm install
npm start
```

## What it demonstrates

- Creating a stowryjs client
- Uploading files with presigned PUT URLs
- Downloading files with presigned GET URLs
- Generating presigned URLs for GET, PUT, DELETE
- Deleting files with presigned DELETE URLs
