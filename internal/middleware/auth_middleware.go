package middleware

import (
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

// Use env variable for JWT secret
func getJWTSecret() []byte {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		// Fallback for tests/development
		return []byte("supersecret")
	}
	return []byte(secret)
}

// AuthMiddleware validates JWT token and extracts user details
func AuthMiddleware(c *fiber.Ctx) error {
	// Get the Authorization header
	tokenString := c.Get("Authorization")
	if tokenString == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Missing token"})
	}

	// Ensure it's a Bearer token
	tokenString = strings.TrimPrefix(tokenString, "Bearer ")
	if tokenString == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token format"})
	}

	// Parse JWT token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return getJWTSecret(), nil
	})
	if err != nil || !token.Valid {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
	}

	// Extract claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token claims"})
	}

	// Retrieve user ID and role from token
	userID, userExists := claims["user_id"].(string)
	role, roleExists := claims["role"].(string)

	if !userExists || !roleExists {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token payload"})
	}

	// Store user info in context for next handlers
	c.Locals("user_id", userID)
	c.Locals("role", role)

	return c.Next()
}
