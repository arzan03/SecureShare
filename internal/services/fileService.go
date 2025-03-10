package services

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/arzan03/SecureShare/internal/db"
	"github.com/arzan03/SecureShare/internal/models"
	"github.com/arzan03/SecureShare/internal/storage"
	"github.com/gofiber/fiber/v2"
	"github.com/minio/minio-go/v7"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func generateSecureToken() (string, error) {
	token := make([]byte, 16)
	_, err := rand.Read(token)
	if err != nil {
		return "", fmt.Errorf("failed to generate secure token: %w", err)
	}
	return hex.EncodeToString(token), nil
}

func UploadFile(c *fiber.Ctx, userID string) (models.File, error) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return models.File{}, errors.New("failed to retrieve file")
	}

	file, err := fileHeader.Open()
	if err != nil {
		return models.File{}, errors.New("failed to open file")
	}
	defer file.Close()

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return models.File{}, errors.New("failed to read file")
	}

	fileID := primitive.NewObjectID()
	bucketName := "secure-files"
	objectName := fmt.Sprintf("%s_%s", fileID.Hex(), fileHeader.Filename)

	_, err = storage.MinioClient.PutObject(
		context.Background(),
		bucketName,
		objectName,
		bytes.NewReader(fileBytes),
		int64(len(fileBytes)),
		minio.PutObjectOptions{ContentType: fileHeader.Header.Get("Content-Type")},
	)
	if err != nil {
		return models.File{}, errors.New("failed to upload file")
	}

	secureToken, err := generateSecureToken()
	if err != nil {
		return models.File{}, errors.New("failed to generate secure token")
	}

	fileData := models.File{
		ID:            fileID,
		Filename:      fileHeader.Filename,
		URL:           fmt.Sprintf("http://%s/%s/%s", os.Getenv("MINIO_ENDPOINT"), bucketName, objectName),
		Owner:         userID,
		ExpiresAt:     time.Now().Add(24 * time.Hour),
		CreatedAt:     time.Now(),
		DownloadToken: secureToken,
		TokenType:     "time-limited",
		TokenExpires:  time.Now().Add(24 * time.Hour),
	}

	collection := db.GetCollection("secure_files", "files")
	_, err = collection.InsertOne(context.TODO(), fileData)
	if err != nil {
		return models.File{}, errors.New("failed to save file metadata")
	}

	return fileData, nil
}


// GeneratePresignedURL creates a presigned URL with security measures.
func GeneratePresignedURL(fileID, userID, tokenType string, duration time.Duration) (string, error) {
	objID, err := primitive.ObjectIDFromHex(fileID)
	if err != nil {
		return "", fmt.Errorf("invalid file ID: %w", err)
	}

	collection := db.GetCollection("secure_files", "files")
	var fileData models.File

	err = collection.FindOne(context.TODO(), bson.M{"_id": objID}).Decode(&fileData)
	if err != nil {
		return "", fmt.Errorf("file not found: %w", err)
	}

	if fileData.Owner != userID {
		return "", errors.New("unauthorized access")
	}

	token, err := generateSecureToken()
	if err != nil {
		return "", err
	}

	// Adjust expiration based on token type
	tokenExpires := time.Now().Add(duration)
	if tokenType == "one-time" {
		tokenExpires = time.Now().Add(30 * time.Minute)
	}

	_, err = collection.UpdateOne(
		context.TODO(),
		bson.M{"_id": objID},
		bson.M{"$set": bson.M{
			"download_token": token,
			"token_type":     tokenType,
			"token_expires":  tokenExpires,
		}},
	)
	if err != nil {
		return "", fmt.Errorf("failed to save download token: %w", err)
	}

	bucketName := "secure-files"
	objectName := fmt.Sprintf("%s_%s", fileID, fileData.Filename)
	expiry := duration

	reqParams := map[string][]string{"token": {token}}
	url, err := storage.MinioClient.PresignedGetObject(context.Background(), bucketName, objectName, expiry, reqParams)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return url.String(), nil
}

// ValidateDownload verifies the token and generates a presigned MinIO download link.
func ValidateDownload(fileID, providedToken string) (string, error) {
	objID, err := primitive.ObjectIDFromHex(fileID)
	if err != nil {
		return "", fmt.Errorf("invalid file ID: %w", err)
	}

	collection := db.GetCollection("secure_files", "files")
	var fileData models.File

	err = collection.FindOne(context.TODO(), bson.M{"_id": objID}).Decode(&fileData)
	if err != nil {
		return "", fmt.Errorf("file not found: %w", err)
	}

	if fileData.DownloadToken != providedToken {
		return "", errors.New("invalid or expired download token")
	}

	if fileData.TokenType == "time-limited" && time.Now().After(fileData.TokenExpires) {
		return "", errors.New("download token expired")
	}

	// Revoke one-time token after first use
	if fileData.TokenType == "one-time" {
		_, err = collection.UpdateOne(
			context.TODO(),
			bson.M{"_id": objID},
			bson.M{"$unset": bson.M{"download_token": ""}}, // `unset` is better than setting empty string
		)
		if err != nil {
			return "", fmt.Errorf("failed to revoke token: %w", err)
		}
	}

	// Generate MinIO presigned URL
	bucketName := "secure-files"
	objectName := fmt.Sprintf("%s_%s", fileID, fileData.Filename)
	expiry := 10 * time.Minute

	url, err := storage.MinioClient.PresignedGetObject(context.Background(), bucketName, objectName, expiry, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate download link: %w", err)
	}

	return url.String(), nil
}