package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/arzan03/SecureShare/internal/db"
	"github.com/arzan03/SecureShare/internal/storage"
	"github.com/gofiber/fiber/v2"
	"github.com/minio/minio-go/v7"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// File Metadata Model
type File struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Filename  string             `bson:"filename" json:"filename"`
	URL       string             `bson:"url" json:"url"`
	Owner     string             `bson:"owner" json:"owner"`
	ExpiresAt time.Time          `bson:"expires_at" json:"expires_at"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
}

// UploadFile uploads a file to MinIO and saves metadata in MongoDB
func UploadFile(c *fiber.Ctx, userID string) (File, error) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return File{}, errors.New("failed to retrieve file")
	}

	// Open file
	file, err := fileHeader.Open()
	if err != nil {
		return File{}, errors.New("failed to open file")
	}
	defer file.Close()

	// Read file content
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return File{}, errors.New("failed to read file")
	}

	// Generate file ID & filename
	fileID := primitive.NewObjectID()
	bucketName := "secure-files"
	objectName := fmt.Sprintf("%s_%s", fileID.Hex(), fileHeader.Filename)

	// Upload file to MinIO
	_, err = storage.MinioClient.PutObject(
		context.Background(),
		bucketName,
		objectName,
		bytes.NewReader(fileBytes),
		int64(len(fileBytes)),
		minio.PutObjectOptions{ContentType: fileHeader.Header.Get("Content-Type")},
	)

	if err != nil {
		return File{}, errors.New("failed to upload file")
	}

	// File Metadata
	fileData := File{
		ID:        fileID,
		Filename:  fileHeader.Filename,
		URL:       fmt.Sprintf("http://%s/%s/%s", os.Getenv("MINIO_ENDPOINT"), bucketName, objectName),
		Owner:     userID,
		ExpiresAt: time.Now().Add(24 * time.Hour), // Default expiration: 24 hours
		CreatedAt: time.Now(),
	}

	// Save metadata to MongoDB
	collection := db.GetCollection("secure_files", "files")
	_, err = collection.InsertOne(context.TODO(), fileData)
	if err != nil {
		return File{}, errors.New("failed to save file metadata")
	}

	return fileData, nil
}

// GeneratePresignedURL generates a temporary download link for a file
func GeneratePresignedURL(fileID string, userID string) (string, error) {
	// Convert fileID to MongoDB ObjectID
	objID, err := primitive.ObjectIDFromHex(fileID)
	if err != nil {
		return "", errors.New("invalid file ID")
	}

	// Fetch file metadata from MongoDB
	collection := db.GetCollection("secure_files", "files")
	var fileData struct {
		Filename string `bson:"filename"`
		Owner    string `bson:"owner"`
	}
	err = collection.FindOne(context.TODO(), bson.M{"_id": objID}).Decode(&fileData)
	if err != nil {
		return "", errors.New("file not found")
	}

	// Ensure user is the owner
	if fileData.Owner != userID {
		return "", errors.New("unauthorized access")
	}

	// Generate MinIO pre-signed URL
	bucketName := "secure-files"
	objectName := fmt.Sprintf("%s_%s", fileID, fileData.Filename)
	expiry := time.Minute * 10 // Link valid for 10 minutes

	// Generate URL
	reqParams := make(map[string][]string) // Ensure correct format
	url, err := storage.MinioClient.PresignedGetObject(context.Background(), bucketName, objectName, expiry, reqParams)
	if err != nil {
		return "", errors.New("failed to generate download link")
	}

	return url.String(), nil
}
