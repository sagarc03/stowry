# Python AWS SDK (boto3) Example

Demonstrates using Stowry with [boto3](https://boto3.amazonaws.com/v1/documentation/api/latest/index.html) and AWS Signature V4.

## Prerequisites

- Python 3.11+
- Stowry server running

## Run

```bash
# Start Stowry
stowry serve --config ../config.yaml --db-dsn /tmp/stowry.db --storage-path /tmp/data

# Run example
cd examples/python-aws
python -m venv .venv
source .venv/bin/activate  # Windows: .venv\Scripts\activate
pip install -r requirements.txt
python main.py
```

## What it demonstrates

- Configuring boto3 for S3-compatible endpoints
- Using path-style addressing (`addressing_style: "path"`)
- Setting signature version (`signature_version="s3v4"`)
- Generating presigned URLs with `generate_presigned_url()`
- Upload, download, and delete operations via presigned URLs

## Notes

- Stowry only supports query-string authentication (presigned URLs)
- The bucket name in the path is required but can be any value
