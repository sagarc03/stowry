import { StowryClient } from "stowryjs";

function getStowryClient(): StowryClient {
  const endpoint = process.env.STOWRY_SECURE === "true" ? "https://" : "http://";
  const host = process.env.STOWRY_HOST;
  const accessKey = process.env.STOWRY_ACCESS_KEY;
  const secretKey = process.env.STOWRY_SECRET_KEY;

  if (!host || !accessKey || !secretKey) {
    throw new Error(
      "Missing required environment variables: STOWRY_HOST, STOWRY_ACCESS_KEY, STOWRY_SECRET_KEY"
    );
  }

  return new StowryClient({
    endpoint: endpoint + host,
    accessKey,
    secretKey,
  });
}

export async function getDownloadUrl(key: string, expires = 900): Promise<string> {
  const client = getStowryClient();
  const bucket = process.env.STOWRY_BUCKET || "uploads";
  return client.presignGet(`/${bucket}/${key}`, expires);
}
