# SecureShare

SecureShare is a secure file sharing API built with Go, designed for efficient and secure file management through a RESTful interface. It provides robust authentication, parallel processing for batch operations, and secure access controls.

## Features

- **Secure Authentication**: JWT-based authentication system with role-based access control
- **Efficient File Operations**: 
  - Upload and store files securely
  - Generate presigned URLs for secure file sharing
  - Parallel processing for batch operations
  - Support for both one-time and time-limited access tokens
- **Admin Management**: Administrative controls for user and file management
- **Containerized Deployment**: Docker and docker-compose support for easy deployment
- **Object Storage Integration**: MinIO integration for scalable object storage
- **Database**: MongoDB for metadata storage and user management

## Architecture

SecureShare uses a modern, scalable architecture:

- **Backend**: Go with Fiber web framework
- **Authentication**: JWT tokens with role-based permissions
- **Storage**: 
  - MinIO for file object storage
  - MongoDB for user data and file metadata
- **Containerization**: Docker and docker-compose for deployment

## Prerequisites

- Go 1.21+
- MongoDB
- MinIO
- Docker and docker-compose (for containerized deployment)

## Installation

### Local Development

1. Clone the repository:
   ```bash
   git clone https://github.com/arzan03/SecureShare.git
   cd SecureShare
   ```

2. Install dependencies:
   ```bash
   go mod download
   ```

3. Create and configure your environment variables:
   ```bash
   cp .env.example .env
   # Edit .env with your configuration
   ```

4. Start the server:
   ```bash
   go run cmd/main.go
   ```

### Docker Deployment

1. With docker-compose:
   ```bash
   docker-compose up -d
   ```

## Environment Variables

Configure the following environment variables in your `.env` file:

```
# API Configuration
JWT_SECRET=change_this_in_production

# MongoDB Configuration
MONGO_URI=mongodb://localhost:27017/secure_files

# MinIO Configuration
MINIO_ENDPOINT=localhost:9000
MINIO_ACCESS_KEY=minioadmin
MINIO_SECRET_KEY=minioadmin

# Server Configuration
PORT=8080
```

## API Endpoints

### Authentication
- `POST /auth/register` - Register a new user
- `POST /auth/login` - Login and get JWT token

### Admin Routes
- `GET /admin/users` - List all users
- `GET /admin/files` - List all files
- `GET /admin/user/:userid` - Get user by ID
- `DELETE /admin/file/:file_id` - Delete file (admin only)

### File Operations
- `POST /file/upload` - Upload a file
- `POST /file/presigned/:id` - Generate presigned URL for a file
- `POST /file/presigned` - Generate presigned URLs for multiple files
- `GET /file/download/:id` - Validate and download a file
- `GET /file/list` - List user's files
- `GET /file/metadata/:id` - Get file metadata
- `DELETE /file/:id` - Delete a file
- `POST /file/delete` - Delete multiple files

## Testing

Run the automated tests:

```bash
go test -v ./tests
```

The project includes comprehensive API tests that verify all endpoints and functionality.

## Security Features

- **Secure Tokens**: Cryptographically secure tokens for file access
- **Time-Limited Access**: Files can be shared with time-limited access controls
- **One-Time Downloads**: Support for one-time download links
- **Parallel Operations**: Secure batch operations with proper access controls

## License

[MIT License](LICENSE)

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
