package main

import (
	"log"
	"os"

	"github.com/arzan03/SecureShare/internal/db"
	"github.com/arzan03/SecureShare/internal/handlers"
	"github.com/arzan03/SecureShare/internal/middleware"
	"github.com/arzan03/SecureShare/internal/storage"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found or error loading it, using environment variables")
	}

	// Initialize Fiber
	app := fiber.New()
	// Initialize MinIO
	storage.InitMinio()
	// Middleware
	app.Use(logger.New())
	app.Use(cors.New())

	// Get MongoDB URI from environment
	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017/secure_files" // Default fallback
	}

	// Connect to MongoDB
	mongoDB := db.ConnectMongoDB(mongoURI, "secure_files")

	handlers.InitAdminHandler(mongoDB)
	// Auth Routes
	auth := app.Group("/auth")
	auth.Post("/register", handlers.RegisterHandler)
	auth.Post("/login", handlers.LoginHandler)

	// Admin Routes
	admin := app.Group("/admin", middleware.AdminMiddleware)
	admin.Get("/users", handlers.ListUsers)
	admin.Get("/files", handlers.ListAllFiles)
	admin.Get("/user/:userid", handlers.GetUserByID)
	admin.Delete("/file/:file_id", handlers.AdminDeleteFile)

	// File Routes
	file := app.Group("/file", middleware.AuthMiddleware)
	file.Post("/upload", handlers.UploadFileHandler)

	// URL generation - both endpoints point to the same handler now
	file.Post("/presigned/:id", handlers.GeneratePresignedURLHandler) // Single file with ID in URL
	file.Post("/presigned", handlers.GeneratePresignedURLHandler)     // Handles both single and batch requests from body

	file.Get("/download/:id", handlers.ValidateDownloadHandler)
	file.Get("/list", handlers.ListUserFilesHandler)
	file.Get("/metadata/:id", handlers.GetFileMetadataHandler)

	// Deletion endpoints - both use same handler now
	file.Delete("/:id", handlers.DeleteFileHandler)  // Single deletion with ID in URL
	file.Post("/delete", handlers.DeleteFileHandler) // Handles both single and batch deletions from body

	// Get port from environment
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Default port
	}

	// Start server
	log.Fatal(app.Listen(":" + port))
}
