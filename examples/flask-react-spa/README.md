# Flask + React SPA Example

A file upload example using Flask backend with stowrypy for presigned URL generation and a React SPA frontend.

## Architecture

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  React SPA      │────▶│  Flask API      │────▶│  Stowry         │
│  (Vite)         │     │  (stowrypy)     │     │  (Object Store) │
│  Port 5173      │     │  Port 8080      │     │  Port 5708      │
└─────────────────┘     └─────────────────┘     └─────────────────┘
        │                                               ▲
        │                                               │
        └───────────────────────────────────────────────┘
                    Direct upload via presigned URL
```

## Prerequisites

- Python 3.10+
- Node.js 20+
- Stowry running locally

## Setup

### 1. Start Stowry

```bash
# From the stowry root directory
./stowry serve --config examples/config.yaml --db-dsn /tmp/stowry.db --storage-path /tmp/data
```

### 2. Start Flask Backend

```bash
cd backend
python -m venv .venv
source .venv/bin/activate  # On Windows: .venv\Scripts\activate
pip install -r requirements.txt
cp .env.example .env
python app.py
```

### 3. Start React Frontend

```bash
cd frontend
npm install
npm run dev
```

### 4. Open the App

Navigate to http://localhost:5173 in your browser.

## How It Works

1. **User selects files** in the React frontend
2. **React requests presigned URL** from Flask backend (`POST /api/presign/upload`)
3. **Flask uses stowrypy** to generate a presigned PUT URL with Stowry's native signing
4. **React uploads directly to Stowry** using the presigned URL
5. **React saves metadata** to Flask backend (`POST /api/files`)
6. **File list displays** with download links (also using presigned URLs)

## Configuration

### Backend (.env)

```env
STOWRY_ENDPOINT=http://localhost:5708
STOWRY_ACCESS_KEY=FE373CEF5632FDED3081
STOWRY_SECRET_KEY=9218d0ddfdb1779169f4b6b3b36df321099e98e9
STOWRY_BUCKET=uploads
```

### Frontend (.env)

```env
VITE_API_URL=http://localhost:8080
```

## API Endpoints

### Flask Backend

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/presign/upload` | Generate presigned upload URL |
| POST | `/api/files` | Save file metadata |
| GET | `/api/files` | List files with download URLs |
| GET | `/api/presign/download/<key>` | Generate presigned download URL |
