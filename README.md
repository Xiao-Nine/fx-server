# fx-server

`fx-server` is the backend service for the **File Express (fx)** project. It provides a lightweight API for anonymous file uploading and downloading via pickup codes.

## Features
- **Anonymous File Management**: Upload files and receive a unique pickup code (no auth).
- **Flexible Options**: Support for one-time downloads (burn-after-reading), file expirations, and never-expire uploads.
- **Automatic Cleanup**: Periodically deletes expired files and their DB records.
- **Lightweight Storage**: Uses SQLite for storing file metadata; stores file contents on disk under `data/uploads/`.
- **Configurable Limits**: Set maximum file upload size via environment variable.
- **Rate Limiting**: Protects against abuse with 5 requests per minute per IP limit.

## Getting Started

### Prerequisites
- Go 1.21 or higher

### Configuration

#### Environment Variables
- `MAX_FILE_SIZE`: Maximum file upload size in bytes (default: 100MB = 104857600 bytes)
  - Example: `MAX_FILE_SIZE=2147483648` for 2GB limit

### Installation & Run

#### Option 1: Direct Run
1. Navigate to the server directory:
   ```bash
   cd fx-server
   ```
2. Run the server:
   ```bash
   go run main.go
   ```
   The server will start on port `8080` by default and create an SQLite database in the `data/` directory.

#### Option 2: Docker Compose
1. Build and run with Docker Compose:
   ```bash
   docker-compose up -d
   ```
2. To configure max file size, edit `docker-compose.yml`:
   ```yaml
   services:
     fx-server:
       environment:
         - MAX_FILE_SIZE=2147483648  # 2GB
   ```

## Pickup Code Uniqueness

The system guarantees unique 6-digit pickup codes by:
- Generating random codes in the range 100000-999999
- Checking database for collisions before assignment
- Retrying up to 100 times if collision occurs
- Returns error if unable to generate unique code after retries

This ensures no two files share the same pickup code.

## Rate Limiting

The server implements IP-based rate limiting:
- **Limit**: 5 requests per minute per IP address
- **Scope**: Applies to all `/api/*` endpoints (upload and download)
- **Behavior**: Returns HTTP 429 (Too Many Requests) when limit exceeded
- **Cleanup**: Automatically removes expired rate limit records every minute

This protects the server from abuse and ensures fair resource usage.

## API Endpoints

### `POST /api/files/upload`
Multipart form fields:
- `file` (required): the file to upload
- `expire` (optional): Go duration, e.g. `30m`, `24h` (default `24h`)
- `one_time` (optional): `true/1/yes` to burn after first download
- `never_expire` (optional): `true/1/yes` to disable expiration

Returns JSON:
- `code`: pickup code
- `filename`
- `expiresAt`: RFC3339 string or `null`

### `GET /api/files/download/:code`
Downloads the file. Returns:
- `200`: file attachment
- `404`: code not found / file missing
- `410`: expired

Notes:
- Cleanup runs every ~15 minutes and also once at startup.

