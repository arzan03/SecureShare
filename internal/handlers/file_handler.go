package handlers

import (
	"fmt"
	"time"

	"github.com/arzan03/SecureShare/internal/services"
	"github.com/gofiber/fiber/v2"
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
