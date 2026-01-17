# Python Native Signing Example

Demonstrates using Stowry with [stowrypy](https://pypi.org/project/stowrypy/) SDK and native signing.

## Prerequisites

- Python 3.8+
- Stowry server running

## Run

```bash
# Start Stowry
stowry serve --config ../config.yaml --db-dsn /tmp/stowry.db --storage-path /tmp/data

# Run example
cd examples/python-native
python -m venv .venv
source .venv/bin/activate  # Windows: .venv\Scripts\activate
pip install -r requirements.txt
python main.py
```

## What it demonstrates

- Creating a stowrypy client
- Uploading files with presigned PUT URLs
- Downloading files with presigned GET URLs
- Generating presigned URLs for GET, PUT, DELETE
- Deleting files with presigned DELETE URLs
