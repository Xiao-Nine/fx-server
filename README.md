# fx-server

`fx-server` is the backend service for the **File Express (fx)** project. It provides a lightweight API for file uploading, downloading via pickup codes, and user authentication.

## Features
- **User Authentication**: Email-based registration with OTP verification and JWT login.
- **File Management**: Upload files and receive a unique pickup code.
- **Flexible Options**: Support for one-time downloads (burn after reading) and file expirations.
- **Lightweight Storage**: Uses SQLite for storing user data and file metadata.

## Getting Started

### Prerequisites
- Go 1.21 or higher

### Installation & Run
1. Navigate to the server directory:
   ```bash
   cd fx-server
   ```
2. Run the server:
   ```bash
   go run main.go
   ```
   The server will start on port `8080` by default and create an SQLite database in the `data/` directory.

## API Endpoints
- `POST /api/auth/register` - Register a new account
- `POST /api/auth/verify` - Verify email with OTP
- `POST /api/auth/login` - Login to get JWT token
- `POST /api/files/upload` - Upload a file/directory
- `GET /api/files/download/:code` - Download a file using the pickup code
