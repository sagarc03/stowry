/**
 * Example: Using Stowry with presigned URLs (JavaScript)
 *
 * Run Stowry first:
 *   stowry serve --config ../config.yaml
 *
 * Then run this example:
 *   cd examples/javascript-aws
 *   npm install
 *   npm start
 *
 * Requires Node.js 22+ (LTS)
 */

import { readFileSync } from "fs";
import { parse } from "yaml";
import {
  S3Client,
  PutObjectCommand,
  GetObjectCommand,
  DeleteObjectCommand,
} from "@aws-sdk/client-s3";
import { getSignedUrl } from "@aws-sdk/s3-request-presigner";

const STOWRY_ENDPOINT = "http://localhost:5708";
const CONFIG_PATH = "../config.yaml";
const BUCKET = "example";

function loadConfig(path) {
  const content = readFileSync(path, "utf8");
  return parse(content);
}

function createClient(config) {
  const auth = config.auth || {};
  const keys = auth.keys?.inline || [];
  if (keys.length === 0) {
    throw new Error("No auth keys found in config");
  }

  const region = auth.aws?.region || "us-east-1";
  return new S3Client({
    endpoint: STOWRY_ENDPOINT,
    region: region,
    credentials: {
      accessKeyId: keys[0].access_key,
      secretAccessKey: keys[0].secret_key,
    },
    forcePathStyle: true,
  });
}

async function presignGet(client, key, expiresIn = 900) {
  const command = new GetObjectCommand({ Bucket: BUCKET, Key: key });
  return await getSignedUrl(client, command, { expiresIn });
}

async function presignPut(client, key, contentType, expiresIn = 900) {
  const command = new PutObjectCommand({
    Bucket: BUCKET,
    Key: key,
    ContentType: contentType,
  });
  return await getSignedUrl(client, command, { expiresIn });
}

async function presignDelete(client, key, expiresIn = 900) {
  const command = new DeleteObjectCommand({ Bucket: BUCKET, Key: key });
  return await getSignedUrl(client, command, { expiresIn });
}

async function main() {
  const config = loadConfig(CONFIG_PATH);
  const client = createClient(config);

  const key = "hello.txt";
  const content = "Hello from JavaScript presigned URLs!";
  const contentType = "text/plain";

  // Upload using presigned URL
  console.log("=== Upload ===");
  const uploadUrl = await presignPut(client, key, contentType);
  let resp = await fetch(uploadUrl, {
    method: "PUT",
    body: content,
    headers: { "Content-Type": contentType },
  });
  if (!resp.ok) throw new Error(`Upload failed: ${resp.status}`);
  console.log(`Uploaded: ${BUCKET}/${key}`);

  // Download using presigned URL
  console.log("\n=== Download ===");
  const downloadUrl = await presignGet(client, key);
  resp = await fetch(downloadUrl);
  if (!resp.ok) throw new Error(`Download failed: ${resp.status}`);
  const downloaded = await resp.text();
  console.log(`Content: ${downloaded}`);

  // Show presigned URLs
  console.log("\n=== Presigned URLs ===");
  console.log(`GET URL: ${downloadUrl}`);
  console.log(`PUT URL: ${uploadUrl}`);
  const deleteUrl = await presignDelete(client, key);
  console.log(`DELETE URL: ${deleteUrl}`);

  // Delete using presigned URL
  console.log("\n=== Delete ===");
  resp = await fetch(deleteUrl, { method: "DELETE" });
  if (!resp.ok) throw new Error(`Delete failed: ${resp.status}`);
  console.log(`Deleted: ${BUCKET}/${key}`);
}

main().catch(console.error);
