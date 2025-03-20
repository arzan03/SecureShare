package storage

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var MinioClient *minio.Client

func InitMinio() {
	endpoint := os.Getenv("MINIO_ENDPOINT")
	if endpoint == "" {
		endpoint = "localhost:9000" // Default fallback
	}

	accessKey := os.Getenv("MINIO_ACCESS_KEY")
	if accessKey == "" {
		accessKey = "minioadmin" // Default fallback
	}

	secretKey := os.Getenv("MINIO_SECRET_KEY")
	if secretKey == "" {
		secretKey = "minioadmin" // Default fallback
	}

	useSSL := false // Set to true if using HTTPS

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})

	if err != nil {
		log.Fatalf("Failed to connect to MinIO: %v", err)
	}

	// Create a context with timeout for operations
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create required buckets if they don't exist
	bucketName := "secure-files"
	exists, err := client.BucketExists(ctx, bucketName)
	if err != nil {
		log.Printf("Warning: Failed to check bucket existence: %v", err)
	} else if !exists {
		err = client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
		if err != nil {
			log.Printf("Warning: Failed to create bucket: %v", err)
		} else {
			log.Printf("Created bucket: %s", bucketName)
		}
	}

	MinioClient = client
	fmt.Println("✅ Connected to MinIO")
}
