package handlers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/arzan03/SecureShare/internal/models"
	"github.com/arzan03/SecureShare/internal/services"
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// UploadFileHandler handles file uploads
func UploadFileHandler(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string) // Extract user ID from JWT middleware

	fileData, err := services.UploadFile(c, userID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"message": "File uploaded successfully",
		"file":    fileData,
	})
}

// BatchPresignedURLHandler generates multiple presigned URLs in parallel
func BatchPresignedURLHandler(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid user"})
	}

	var requestBody struct {
		FileIDs   []string `json:"file_ids"`
		TokenType string   `json:"token_type"`
		Duration  int      `json:"duration,omitempty"`
	}

	if err := c.BodyParser(&requestBody); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if len(requestBody.FileIDs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No file IDs provided"})
	}

	if requestBody.TokenType != "one-time" && requestBody.TokenType != "time-limited" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid token type"})
	}

	duration := time.Duration(requestBody.Duration) * time.Minute
	if requestBody.TokenType == "one-time" || requestBody.Duration <= 0 {
		duration = 30 * time.Minute
	}

	urls, errs := services.BatchGeneratePresignedURLs(requestBody.FileIDs, userID, requestBody.TokenType, duration)

	return c.JSON(fiber.Map{
		"presigned_urls": urls,
		"errors":         errs,
		"expires_in":     fmt.Sprintf("%d minutes", requestBody.Duration),
	})
}

// GeneratePresignedURLHandler generates presigned URLs for one or multiple files
func GeneratePresignedURLHandler(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid user"})
	}

	// Structure that can handle both single and batch requests
	var requestBody struct {
		FileID    string   `json:"file_id"`
		FileIDs   []string `json:"file_ids"`
		TokenType string   `json:"token_type"`
		Duration  int      `json:"duration,omitempty"`
	}

	if err := c.BodyParser(&requestBody); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Set default token type if not provided
	if requestBody.TokenType != "one-time" && requestBody.TokenType != "time-limited" {
		requestBody.TokenType = "time-limited"
	}

	// Set duration
	duration := time.Duration(requestBody.Duration) * time.Minute
	if requestBody.TokenType == "one-time" || requestBody.Duration <= 0 {
		duration = 30 * time.Minute
	}

	// Determine if it's a batch request or single file request
	if len(requestBody.FileIDs) > 0 {
		// Batch processing
		urls, errs := services.BatchGeneratePresignedURLs(requestBody.FileIDs, userID, requestBody.TokenType, duration)
		return c.JSON(fiber.Map{
			"presigned_urls": urls,
			"errors":         errs,
			"expires_in":     fmt.Sprintf("%d minutes", requestBody.Duration),
		})
	} else {
		// Single file processing - check if from path parameter or body
		fileID := c.Params("id")
		if fileID == "" {
			fileID = requestBody.FileID
		}

		if fileID == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No file ID provided"})
		}

		presignedURL, err := services.GeneratePresignedURL(fileID, userID, requestBody.TokenType, duration)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(fiber.Map{
			"presigned_url": presignedURL,
			"expires_in":    fmt.Sprintf("%d minutes", requestBody.Duration),
		})
	}
}

func ValidateDownloadHandler(c *fiber.Ctx) error {
	fileID := c.Params("id")
	token := c.Query("token")
	if token == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing download token"})
	}

	downloadURL, err := services.ValidateDownload(fileID, token)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"download_url": downloadURL,
		"expires_in":   "10 minutes",
	})
}

// ListUserFilesHandler gets all files uploaded by user using parallel metadata fetching
func ListUserFilesHandler(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	files, err := services.ListFilesWithMetadata(userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(files)
}

// DeleteFileHandler handles both single and batch file deletions
func DeleteFileHandler(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	// Check if this is a batch deletion request
	var requestBody struct {
		FileID  string   `json:"file_id"`
		FileIDs []string `json:"file_ids"`
	}

	// Try to parse a request body (for batch operations)
	bodyErr := c.BodyParser(&requestBody)

	// Single file deletion (from path parameter)
	fileID := c.Params("id")
	if fileID != "" {
		err := services.DeleteFileParallel(fileID, userID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
		return c.JSON(fiber.Map{"message": "File deleted successfully"})
	}

	// Handle body-based requests
	if bodyErr != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request format"})
	}

	// Single file deletion from request body
	if requestBody.FileID != "" {
		err := services.DeleteFileParallel(requestBody.FileID, userID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
		return c.JSON(fiber.Map{"message": "File deleted successfully"})
	}

	// Batch file deletion
	if len(requestBody.FileIDs) > 0 {
		results := make(map[string]string)
		var wg sync.WaitGroup
		resultsMutex := sync.RWMutex{}

		// Process deletions in parallel
		wg.Add(len(requestBody.FileIDs))
		for _, fid := range requestBody.FileIDs {
			go func(fileID string) {
				defer wg.Done()
				err := services.DeleteFileParallel(fileID, userID)

				resultsMutex.Lock()
				if err != nil {
					results[fileID] = fmt.Sprintf("Error: %s", err.Error())
				} else {
					results[fileID] = "Deleted successfully"
				}
				resultsMutex.Unlock()
			}(fid)
		}
		wg.Wait()

		return c.JSON(fiber.Map{"results": results})
	}

	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No file ID provided"})
}

// BatchDeleteFilesHandler deletes multiple files in parallel
func BatchDeleteFilesHandler(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	var requestBody struct {
		FileIDs []string `json:"file_ids"`
	}

	if err := c.BodyParser(&requestBody); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if len(requestBody.FileIDs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No file IDs provided"})
	}

	results := make(map[string]string)
	for _, fileID := range requestBody.FileIDs {
		err := services.DeleteFileParallel(fileID, userID)
		if err != nil {
			results[fileID] = fmt.Sprintf("Error: %s", err.Error())
		} else {
			results[fileID] = "Deleted successfully"
		}
	}

	return c.JSON(fiber.Map{
		"results": results,
	})
}

// GetFileMetadataHandler gets metadata of a single file
func GetFileMetadataHandler(c *fiber.Ctx) error {
	fileID := c.Params("id")
	userID := c.Locals("user_id").(string) // Extract user ID from JWT

	// Convert fileID to MongoDB ObjectID
	objID, err := primitive.ObjectIDFromHex(fileID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid file ID",
		})
	}

	// Query MongoDB for file metadata
	var file models.File
	err = fileCollection.FindOne(context.TODO(), bson.M{"_id": objID, "owner": userID}).Decode(&file)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "File not found or access denied",
		})
	}

	return c.JSON(file)
}
