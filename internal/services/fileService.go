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
	"sync"
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

	// Create channels for parallel execution results
	minioResultChan := make(chan error, 1)
	metadataResultChan := make(chan struct {
		fileData models.File
		err      error
	}, 1)

	// Generate secure token for metadata
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

	// Execute file upload and metadata creation in parallel
	go func() {
		_, err := storage.MinioClient.PutObject(
			context.Background(),
			bucketName,
			objectName,
			bytes.NewReader(fileBytes),
			int64(len(fileBytes)),
			minio.PutObjectOptions{ContentType: fileHeader.Header.Get("Content-Type")},
		)
		minioResultChan <- err
	}()

	go func() {
		collection := db.GetCollection("secure_files", "files")
		_, err := collection.InsertOne(context.TODO(), fileData)
		metadataResultChan <- struct {
			fileData models.File
			err      error
		}{fileData, err}
	}()

	// Wait for both operations to complete
	minioErr := <-minioResultChan
	metadataResult := <-metadataResultChan

	if minioErr != nil {
		return models.File{}, errors.New("failed to upload file to storage: " + minioErr.Error())
	}

	if metadataResult.err != nil {
		// Try to clean up the uploaded file if metadata creation fails
		go func() {
			storage.MinioClient.RemoveObject(context.Background(), bucketName, objectName, minio.RemoveObjectOptions{})
		}()
		return models.File{}, errors.New("failed to save file metadata: " + metadataResult.err.Error())
	}

	return fileData, nil
}

// BatchGeneratePresignedURLs processes multiple files in parallel
func BatchGeneratePresignedURLs(fileIDs []string, userID string, tokenType string, duration time.Duration) (map[string]string, []error) {
	results := make(map[string]string)
	errs := make([]error, 0)
	resultMutex := sync.RWMutex{}

	var wg sync.WaitGroup
	wg.Add(len(fileIDs))

	for _, fileID := range fileIDs {
		go func(fid string) {
			defer wg.Done()
			url, err := GeneratePresignedURL(fid, userID, tokenType, duration)
			resultMutex.Lock()
			if err != nil {
				errs = append(errs, fmt.Errorf("error for file %s: %w", fid, err))
			} else {
				results[fid] = url
			}
			resultMutex.Unlock()
		}(fileID)
	}

	wg.Wait()
	return results, errs
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

// DeleteFileParallel deletes a file from both MinIO and MongoDB in parallel
func DeleteFileParallel(fileID, userID string) error {
	objID, err := primitive.ObjectIDFromHex(fileID)
	if err != nil {
		return fmt.Errorf("invalid file ID: %w", err)
	}

	collection := db.GetCollection("secure_files", "files")
	var file models.File
	err = collection.FindOne(context.TODO(), bson.M{"_id": objID, "owner": userID}).Decode(&file)
	if err != nil {
		return fmt.Errorf("file not found or access denied: %w", err)
	}

	// Create channels for parallel deletion results
	minioDeleteChan := make(chan error, 1)
	mongoDeleteChan := make(chan error, 1)

	bucketName := "secure-files"
	objectName := fmt.Sprintf("%s_%s", fileID, file.Filename)

	// Delete from MinIO in parallel
	go func() {
		err := storage.MinioClient.RemoveObject(context.TODO(), bucketName, objectName, minio.RemoveObjectOptions{})
		minioDeleteChan <- err
	}()

	// Delete from MongoDB in parallel
	go func() {
		_, err := collection.DeleteOne(context.TODO(), bson.M{"_id": objID})
		mongoDeleteChan <- err
	}()

	// Wait for both operations
	minioErr := <-minioDeleteChan
	mongoErr := <-mongoDeleteChan

	// Handle errors
	if minioErr != nil && mongoErr != nil {
		return fmt.Errorf("failed to delete from both storage and database")
	} else if minioErr != nil {
		return fmt.Errorf("failed to delete from storage: %w", minioErr)
	} else if mongoErr != nil {
		return fmt.Errorf("failed to delete from database: %w", mongoErr)
	}

	return nil
}

// ListFilesWithMetadata gets all files for a user with parallel metadata fetching
func ListFilesWithMetadata(userID string) ([]models.File, error) {
	collection := db.GetCollection("secure_files", "files")

	cursor, err := collection.Find(context.TODO(), bson.M{"owner": userID})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve files: %w", err)
	}
	defer cursor.Close(context.TODO())

	var files []models.File
	if err = cursor.All(context.TODO(), &files); err != nil {
		return nil, fmt.Errorf("error decoding file metadata: %w", err)
	}

	// For each file, fetch additional metadata in parallel if needed
	// This is a placeholder for any additional enrichment that might be needed
	if len(files) > 0 {
		var wg sync.WaitGroup
		wg.Add(len(files))

		for i := range files {
			go func(index int) {
				defer wg.Done()
				// Here you could enrich each file with additional data
				// For example, checking if it exists in MinIO

				// This is just a placeholder, no actual enrichment needed for now
			}(i)
		}

		wg.Wait()
	}

	return files, nil
}
