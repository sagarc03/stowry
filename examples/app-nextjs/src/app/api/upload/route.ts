import { route, type Router } from "@better-upload/server";
import { toRouteHandler } from "@better-upload/server/adapters/next";
import { custom } from "@better-upload/server/clients";

function createRouter(): Router {
  const host = process.env.STOWRY_HOST;
  const accessKeyId = process.env.STOWRY_ACCESS_KEY;
  const secretAccessKey = process.env.STOWRY_SECRET_KEY;

  if (!host || !accessKeyId || !secretAccessKey) {
    throw new Error(
      "Missing required environment variables: STOWRY_HOST, STOWRY_ACCESS_KEY, STOWRY_SECRET_KEY"
    );
  }

  return {
    client: custom({
      host,
      accessKeyId,
      secretAccessKey,
      region: process.env.STOWRY_REGION || "us-east-1",
      secure: process.env.STOWRY_SECURE === "true",
      forcePathStyle: true,
    }),
    bucketName: process.env.STOWRY_BUCKET || "uploads",
    routes: {
      files: route({
        fileTypes: ["image/*", "application/pdf", "text/*"],
        maxFileSize: 10 * 1024 * 1024, // 10MB
      }),
    },
  };
}

export const POST = async (req: Request) => {
  const router = createRouter();
  const handler = toRouteHandler(router);
  return handler.POST(req);
};
