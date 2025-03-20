# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download 

# Add godotenv package
RUN go get github.com/joho/godotenv

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o secure-share ./cmd/main.go

# Final stage
FROM alpine:latest

WORKDIR /app

# Copy binary from build stage
COPY --from=builder /app/secure-share .

# Create dirs for uploads
RUN mkdir -p /app/uploads

# Install CA certificates for HTTPS connections
RUN apk --no-cache add ca-certificates

# Expose port (default, can be overridden by environment)
EXPOSE 8080

# Set default values for required environment variables
ENV JWT_SECRET=changeme_in_production \
    MINIO_ENDPOINT=localhost:9000 \
    MINIO_ACCESS_KEY=minioadmin \
    MINIO_SECRET_KEY=minioadmin \
    PORT=8080 \
    MONGO_URI=mongodb://localhost:27017/secure_files

# Run the application
CMD ["./secure-share"]
