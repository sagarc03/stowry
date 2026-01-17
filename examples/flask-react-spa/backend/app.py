"""Flask backend for file uploads using Stowry with stowrypy."""

import os
import re
import uuid
from flask import Flask, jsonify, request
from flask_cors import CORS
from stowrypy import StowryClient
from dotenv import load_dotenv


def sanitize_filename(filename: str) -> str:
    """Sanitize filename to be safe for URLs and file systems."""
    # Replace spaces with underscores
    name = filename.replace(" ", "_")
    # Remove or replace other problematic characters
    name = re.sub(r"[^\w\-.]", "_", name)
    # Collapse multiple underscores
    name = re.sub(r"_+", "_", name)
    return name

load_dotenv()

app = Flask(__name__)
CORS(app)

# Initialize Stowry client
client = StowryClient(
    endpoint=os.getenv("STOWRY_ENDPOINT", "http://localhost:5708"),
    access_key=os.getenv("STOWRY_ACCESS_KEY", ""),
    secret_key=os.getenv("STOWRY_SECRET_KEY", ""),
)

BUCKET = os.getenv("STOWRY_BUCKET", "uploads")

# In-memory file storage (use a real database in production)
uploaded_files: list[dict] = []


@app.route("/api/presign/upload", methods=["POST"])
def presign_upload():
    """Generate a presigned URL for uploading a file."""
    data = request.json
    if not data:
        return jsonify({"error": "No JSON data provided"}), 400

    filename = data.get("filename")
    if not filename:
        return jsonify({"error": "filename is required"}), 400

    content_type = data.get("content_type", "application/octet-stream")

    # Generate unique key to avoid collisions
    # Sanitize filename to handle special characters
    unique_id = str(uuid.uuid4())[:8]
    safe_filename = sanitize_filename(filename)
    key = f"/{BUCKET}/{unique_id}-{safe_filename}"

    # Generate presigned PUT URL using stowrypy
    url = client.presign_put(key, expires=900)

    return jsonify({
        "url": url,
        "key": key,
        "filename": filename,
        "content_type": content_type,
    })


@app.route("/api/files", methods=["POST"])
def save_file():
    """Save file metadata after successful upload."""
    data = request.json
    if not data:
        return jsonify({"error": "No JSON data provided"}), 400

    file_record = {
        "id": len(uploaded_files) + 1,
        "name": data.get("name", ""),
        "key": data.get("key", ""),
        "size": data.get("size", 0),
        "content_type": data.get("content_type", "application/octet-stream"),
    }
    uploaded_files.append(file_record)

    return jsonify(file_record), 201


@app.route("/api/files", methods=["GET"])
def list_files():
    """List all uploaded files with download URLs."""
    files_with_urls = []
    for file in uploaded_files:
        download_url = client.presign_get(file["key"], expires=900)
        files_with_urls.append({
            **file,
            "download_url": download_url,
        })

    return jsonify(files_with_urls)


@app.route("/api/presign/download/<path:key>", methods=["GET"])
def presign_download(key: str):
    """Generate a presigned URL for downloading a file."""
    # Ensure key starts with /
    if not key.startswith("/"):
        key = "/" + key

    url = client.presign_get(key, expires=900)
    return jsonify({"url": url})


if __name__ == "__main__":
    app.run(port=8080, debug=True)
