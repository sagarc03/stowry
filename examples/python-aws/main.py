#!/usr/bin/env python3
"""
Example: Using Stowry with presigned URLs (Python)

Run Stowry first:
    stowry serve --config ../config.yaml

Then run this example:
    cd examples/python-aws
    python -m venv .venv
    source .venv/bin/activate  # or .venv\\Scripts\\activate on Windows
    pip install -r requirements.txt
    python main.py

Requires Python 3.11+
"""

import boto3
import requests
import yaml
from botocore.config import Config

STOWRY_ENDPOINT = "http://localhost:5708"
CONFIG_PATH = "../config.yaml"
BUCKET = "example"


def load_config(path: str) -> dict:
    """Load configuration from YAML file."""
    with open(path) as f:
        return yaml.safe_load(f)


def create_client(config: dict):
    """Create an S3 client for generating presigned URLs."""
    auth = config.get("auth", {})
    keys = auth.get("keys", [])
    if not keys:
        raise ValueError("No auth keys found in config")

    return boto3.client(
        "s3",
        endpoint_url=STOWRY_ENDPOINT,
        aws_access_key_id=keys[0]["access_key"],
        aws_secret_access_key=keys[0]["secret_key"],
        region_name=auth.get("region", "us-east-1"),
        config=Config(
            s3={"addressing_style": "path"},
            signature_version="s3v4",
        ),
    )


def presign_get(client, key: str, expires_in: int = 900) -> str:
    """Generate a presigned URL for downloading."""
    return client.generate_presigned_url(
        "get_object",
        Params={"Bucket": BUCKET, "Key": key},
        ExpiresIn=expires_in,
    )


def presign_put(client, key: str, content_type: str, expires_in: int = 900) -> str:
    """Generate a presigned URL for uploading."""
    return client.generate_presigned_url(
        "put_object",
        Params={"Bucket": BUCKET, "Key": key, "ContentType": content_type},
        ExpiresIn=expires_in,
    )


def presign_delete(client, key: str, expires_in: int = 900) -> str:
    """Generate a presigned URL for deleting."""
    return client.generate_presigned_url(
        "delete_object",
        Params={"Bucket": BUCKET, "Key": key},
        ExpiresIn=expires_in,
    )


def main():
    config = load_config(CONFIG_PATH)
    client = create_client(config)

    key = "hello.txt"
    content = b"Hello from Python presigned URLs!"
    content_type = "text/plain"

    # Upload using presigned URL
    print("=== Upload ===")
    upload_url = presign_put(client, key, content_type)
    print(upload_url)
    resp = requests.put(
        upload_url, data=content, headers={"Content-Type": content_type}
    )
    resp.raise_for_status()
    print(f"Uploaded: {BUCKET}/{key}")

    # Download using presigned URL
    print("\n=== Download ===")
    download_url = presign_get(client, key)
    resp = requests.get(download_url)
    resp.raise_for_status()
    print(f"Content: {resp.text}")

    # Show presigned URLs
    print("\n=== Presigned URLs ===")
    print(f"GET URL: {download_url}")
    print(f"PUT URL: {upload_url}")
    delete_url = presign_delete(client, key)
    print(f"DELETE URL: {delete_url}")

    # Delete using presigned URL
    print("\n=== Delete ===")
    delete_url = presign_delete(client, key)
    resp = requests.delete(delete_url)
    resp.raise_for_status()
    print(f"Deleted: {BUCKET}/{key}")


if __name__ == "__main__":
    main()
