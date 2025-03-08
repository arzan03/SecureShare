package main

import (
	"log"
	"os"

	"github.com/arzan03/SecureShare/internal/db"
	"github.com/arzan03/SecureShare/internal/handlers"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

func main() {
	// Load environment variables
	os.Setenv("JWT_SECRET", "supersecret") // Change this in production

	// Initialize Fiber
	app := fiber.New()
	// Middleware
	app.Use(logger.New())
	app.Use(cors.New())

	// Connect to MongoDB
	db.ConnectMongoDB("mongodb+srv://arzan03:pass123@go.znpbv.mongodb.net/?retryWrites=true&w=majority&appName=go")

	// Auth Routes
	auth := app.Group("/auth")
	auth.Post("/register", handlers.RegisterHandler)
	auth.Post("/login", handlers.LoginHandler)

	// Start server
	log.Fatal(app.Listen(":8080"))
}
