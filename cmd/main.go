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
)

func main() {
	// Load environment variables
	os.Setenv("JWT_SECRET", "supersecret") // Change this in production
	os.Setenv("MINIO_ENDPOINT", "10.50.36.60:9000")
	os.Setenv("MINIO_ACCESS_KEY", "minioadmin")
	os.Setenv("MINIO_SECRET_KEY", "minioadmin")

	// Initialize Fiber
	app := fiber.New()
	// Initialize MinIO
	storage.InitMinio()
	// Middleware
	app.Use(logger.New())
	app.Use(cors.New())

	// Connect to MongoDB
	mongoDB := db.ConnectMongoDB("mongodb+srv://arzan03:pass123@go.znpbv.mongodb.net/?retryWrites=true&w=majority&appName=go", "secure_files")

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

	// Start server
	log.Fatal(app.Listen(":8080"))
}
