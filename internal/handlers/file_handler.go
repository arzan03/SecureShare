package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/arzan03/SecureShare/internal/models"
	"github.com/arzan03/SecureShare/internal/services"
	"github.com/arzan03/SecureShare/internal/storage"
	"github.com/gofiber/fiber/v2"
	"github.com/minio/minio-go/v7"
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

// GeneratePresignedURLHandler generates a presigned URL based on token type
func GeneratePresignedURLHandler(c *fiber.Ctx) error {
	fileID := c.Params("id")
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid user"})
	}

	var requestBody struct {
		TokenType string `json:"token_type"`
		Duration  int    `json:"duration,omitempty"`
	}
	if err := c.BodyParser(&requestBody); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if requestBody.TokenType != "one-time" && requestBody.TokenType != "time-limited" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid token type"})
	}

	duration := time.Duration(requestBody.Duration) * time.Minute
	if requestBody.TokenType == "one-time" || requestBody.Duration <= 0 {
		duration = 30 * time.Minute
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
//List all the files uploaded by user
func ListUserFilesHandler(c *fiber.Ctx) error {
    userID := c.Locals("user_id").(string)

    var files []models.File
    cursor, err := fileCollection.Find(context.TODO(), bson.M{"owner": userID})
    if err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": "Failed to retrieve files",
        })
    }
    defer cursor.Close(context.TODO())

    if err = cursor.All(context.TODO(), &files); err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": "Error decoding file metadata",
        })
    }

    return c.JSON(files)
}
//Delete the file by user
func DeleteFileHandler(c *fiber.Ctx) error {
    fileID := c.Params("id")
    userID := c.Locals("user_id").(string)

    objID, err := primitive.ObjectIDFromHex(fileID)
    if err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
            "error": "Invalid file ID",
        })
    }

    var file models.File
    err = fileCollection.FindOne(context.TODO(), bson.M{"_id": objID, "owner": userID}).Decode(&file)
    if err != nil {
        return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
            "error": "File not found or access denied",
        })
    }

    // Delete from MinIO
    err = storage.MinioClient.RemoveObject(context.TODO(), "your-bucket", file.Filename, minio.RemoveObjectOptions{})
    if err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": "Failed to delete file from storage",
        })
    }

    // Delete from MongoDB
    _, err = fileCollection.DeleteOne(context.TODO(), bson.M{"_id": objID})
    if err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": "Failed to delete file metadata",
        })
    }

    return c.JSON(fiber.Map{"message": "File deleted successfully"})
}
//Get one file 
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
