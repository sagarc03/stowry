# Next.js + Better Upload Example

This example demonstrates how to use [Better Upload](https://better-upload.com) with Stowry as an S3-compatible backend for direct browser uploads, and [stowryjs](https://www.npmjs.com/package/stowryjs) for generating presigned download URLs.

## Features

- Direct browser-to-Stowry uploads via Better Upload
- File metadata persistence in SQLite database
- Download links generated using stowryjs native signing
- Server Component rendering for file list
- Real-time upload progress UI

## Prerequisites

- Node.js 18+
- Stowry server running with CORS enabled

## Setup

1. **Start Stowry server**

   ```bash
   # From the stowry root directory
   go build ./cmd/stowry
   ./stowry serve --config examples/config.yaml --db-dsn /tmp/stowry.db --storage-path /tmp/data
   ```

2. **Configure environment**

   ```bash
   cp .env.example .env.local
   ```

   Edit `.env.local` if your Stowry server uses different credentials or port.

3. **Install dependencies**

   ```bash
   npm install
   ```

4. **Start the development server**

   ```bash
   npm run dev
   ```

5. **Open the app**

   Navigate to [http://localhost:3000](http://localhost:3000)

## How It Works

### Upload Flow (Better Upload + AWS Sig V4)

1. **Upload API Route** (`src/app/api/upload/route.ts`)
   - Configures Better Upload with Stowry as a custom S3-compatible backend
   - Uses AWS Signature V4 for presigned upload URLs
   - Defines file upload constraints (types, size limits)

2. **Upload Form** (`src/components/upload-form.tsx`)
   - Client component using `useUploadFiles` hook
   - Drag-and-drop and file picker UI
   - Calls server action on upload complete to persist metadata

### Download Flow (stowryjs Native Signing)

1. **Stowry Client** (`src/lib/stowry.ts`)
   - Uses stowryjs SDK with Stowry native signing scheme
   - Generates presigned GET URLs for downloads

2. **Server Component** (`src/app/page.tsx`)
   - Fetches file list from SQLite database
   - Generates presigned download URLs server-side
   - Renders table with download links

### Data Persistence

- **SQLite Database** (`src/lib/db.ts`)
  - Stores file metadata (name, key, size, content type)
  - Server action saves metadata after successful upload
  - Database file stored locally as `uploads.db`

## Project Structure

```
src/
├── app/
│   ├── api/upload/route.ts   # Better Upload API route
│   ├── actions.ts            # Server action for saving file metadata
│   └── page.tsx              # Server component (file list + upload form)
├── components/
│   └── upload-form.tsx       # Client component for uploads
└── lib/
    ├── db.ts                 # SQLite database operations
    └── stowry.ts             # stowryjs client wrapper
```

## Configuration

| Environment Variable | Description | Default |
|---------------------|-------------|---------|
| `STOWRY_HOST` | Stowry server host:port | `localhost:5708` |
| `STOWRY_ACCESS_KEY` | Access key for signing | - |
| `STOWRY_SECRET_KEY` | Secret key for signing | - |
| `STOWRY_REGION` | AWS region (for signature) | `us-east-1` |
| `STOWRY_BUCKET` | Bucket/path prefix | `uploads` |
| `STOWRY_SECURE` | Use HTTPS | `false` |

## Stowry CORS Configuration

Ensure your Stowry config has CORS enabled:

```yaml
cors:
  enabled: true
  allowed_origins:
    - "http://localhost:3000"
  allowed_methods:
    - GET
    - PUT
    - DELETE
    - OPTIONS
  allowed_headers:
    - Content-Type
    - Authorization
    - X-Amz-Date
    - X-Amz-Content-Sha256
  exposed_headers:
    - ETag
    - Content-Length
  max_age: 300
```

## Signing Schemes

This example demonstrates both signing schemes supported by Stowry:

| Feature | Signing Scheme | Library |
|---------|---------------|---------|
| Upload | AWS Signature V4 | Better Upload |
| Download | Stowry Native | stowryjs |

Both schemes work interchangeably with the same access/secret key pair.
