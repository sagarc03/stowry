/**
 * Example: Using Stowry with stowryjs native signing
 *
 * This example demonstrates using presigned URLs with Stowry's native
 * signing scheme via the stowryjs SDK.
 *
 * Run Stowry first:
 *   stowry serve --config ../config.yaml
 *
 * Then run this example:
 *   cd examples/javascript-native
 *   npm install
 *   npm start
 *
 * Requires Node.js 18+
 */

import { readFileSync } from "fs";
import { parse } from "yaml";
import { StowryClient } from "stowryjs";

const STOWRY_ENDPOINT = "http://localhost:5708";
const CONFIG_PATH = "../config.yaml";

function loadConfig(path) {
  const content = readFileSync(path, "utf8");
  return parse(content);
}

async function main() {
  const config = loadConfig(CONFIG_PATH);
  const auth = config.auth || {};
  const keys = auth.keys || [];

  if (keys.length === 0) {
    throw new Error("No auth keys found in config");
  }

  // Create stowryjs client
  const client = new StowryClient({
    endpoint: STOWRY_ENDPOINT,
    accessKey: keys[0].access_key,
    secretKey: keys[0].secret_key,
  });

  // Upload a file
  const key = "/hello.txt";
  const content = "Hello from stowryjs!";
  const contentType = "text/plain";

  console.log("=== Upload ===");
  const uploadUrl = await client.presignPut(key, 900);
  let resp = await fetch(uploadUrl, {
    method: "PUT",
    body: content,
    headers: { "Content-Type": contentType },
  });
  if (!resp.ok) throw new Error(`Upload failed: ${resp.status}`);
  console.log(`Uploaded: ${key}`);

  // Download the file
  console.log("\n=== Download ===");
  const downloadUrl = await client.presignGet(key, 900);
  resp = await fetch(downloadUrl);
  if (!resp.ok) throw new Error(`Download failed: ${resp.status}`);
  const downloaded = await resp.text();
  console.log(`Content: ${downloaded}`);

  // Generate presigned URLs
  console.log("\n=== Presigned URLs ===");
  console.log(`GET URL: ${await client.presignGet(key, 900)}`);
  console.log(`PUT URL: ${await client.presignPut("/presigned-upload.txt", 900)}`);
  console.log(`DELETE URL: ${await client.presignDelete(key, 900)}`);

  // Delete the file
  console.log("\n=== Delete ===");
  const deleteUrl = await client.presignDelete(key, 900);
  resp = await fetch(deleteUrl, { method: "DELETE" });
  if (!resp.ok) throw new Error(`Delete failed: ${resp.status}`);
  console.log(`Deleted: ${key}`);
}

main().catch(console.error);
