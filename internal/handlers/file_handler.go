package handlers

import (
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

// GetPresignedURLHandler generates a temporary public URL for a file
func GetPresignedURLHandler(c *fiber.Ctx) error {
	fileID := c.Params("id")
	userID, ok := c.Locals("user_id").(string) // Extract user ID from JWT middleware

	if !ok || userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid user"})
	}

	presignedURL, err := services.GeneratePresignedURL(fileID, userID) // Pass both fileID & userID
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"presigned_url": presignedURL,
		"expires_in":    "24 hours",
	})
}
