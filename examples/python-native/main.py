#!/usr/bin/env python3
"""
Example: Using Stowry with stowrypy native signing

This example demonstrates using presigned URLs with Stowry's native
signing scheme via the stowrypy SDK.

Run Stowry first:
    stowry serve --config ../config.yaml

Then run this example:
    cd examples/python-native
    python -m venv .venv
    source .venv/bin/activate  # or .venv\\Scripts\\activate on Windows
    pip install -r requirements.txt
    python main.py

Requires Python 3.8+
"""

import requests
import yaml
from stowrypy import StowryClient

STOWRY_ENDPOINT = "http://localhost:5708"
CONFIG_PATH = "../config.yaml"


def load_config(path: str) -> dict:
    with open(path) as f:
        return yaml.safe_load(f)


def main():
    config = load_config(CONFIG_PATH)
    auth = config.get("auth", {})
    keys = auth.get("keys", {}).get("inline", [])

    if not keys:
        raise ValueError("No auth keys found in config")

    # Create stowrypy client
    client = StowryClient(
        endpoint=STOWRY_ENDPOINT,
        access_key=keys[0]["access_key"],
        secret_key=keys[0]["secret_key"],
    )

    # Upload a file
    key = "/hello.txt"
    content = b"Hello from stowrypy!"
    content_type = "text/plain"

    print("=== Upload ===")
    upload_url = client.presign_put(key, expires=900)
    resp = requests.put(upload_url, data=content, headers={"Content-Type": content_type})
    resp.raise_for_status()
    print(f"Uploaded: {key}")

    # Download the file
    print("\n=== Download ===")
    download_url = client.presign_get(key, expires=900)
    resp = requests.get(download_url)
    resp.raise_for_status()
    print(f"Content: {resp.text}")

    # Generate presigned URLs
    print("\n=== Presigned URLs ===")
    print(f"GET URL: {client.presign_get(key, expires=900)}")
    print(f"PUT URL: {client.presign_put('/presigned-upload.txt', expires=900)}")
    print(f"DELETE URL: {client.presign_delete(key, expires=900)}")

    # Delete the file
    print("\n=== Delete ===")
    delete_url = client.presign_delete(key, expires=900)
    resp = requests.delete(delete_url)
    resp.raise_for_status()
    print(f"Deleted: {key}")


if __name__ == "__main__":
    main()
