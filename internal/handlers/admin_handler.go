package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var userCollection *mongo.Collection
var fileCollection *mongo.Collection

// Initialize MongoDB collections
func InitAdminHandler(db *mongo.Database) {
	userCollection = db.Collection("users")
	fileCollection = db.Collection("files")
}

// List all users
func ListUsers(c *fiber.Ctx) error {
	var users []bson.M
	cursor, err := userCollection.Find(context.TODO(), bson.M{})
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch users"})
	}
	defer cursor.Close(context.TODO())
	cursor.All(context.TODO(), &users)
	return c.JSON(users)
}

// List all uploaded files
func ListAllFiles(c *fiber.Ctx) error {
	var files []bson.M
	cursor, err := fileCollection.Find(context.TODO(), bson.M{})
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch files"})
	}
	defer cursor.Close(context.TODO())
	cursor.All(context.TODO(), &files)
	return c.JSON(files)
}

// Get all files uploaded by a specific user
func ListUserFiles(c *fiber.Ctx) error {
	userID := c.Params("user_id")
	var files []bson.M
	cursor, err := fileCollection.Find(context.TODO(), bson.M{"user_id": userID})
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch user files"})
	}
	defer cursor.Close(context.TODO())
	cursor.All(context.TODO(), &files)
	return c.JSON(files)
}

// Get user details by ID
func GetUserByID(c *fiber.Ctx) error {
	userID := c.Params("userid")

	// Convert string userID to MongoDB ObjectID
	objID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid user ID format"})
	}

	var user bson.M
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = userCollection.FindOne(ctx, bson.M{"_id": objID}).Decode(&user)
	if err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	return c.JSON(user)
}

// Force delete a file (Admin Only)
func AdminDeleteFile(c *fiber.Ctx) error {
	fileID := c.Params("file_id")
	result, err := fileCollection.DeleteOne(context.TODO(), bson.M{"_id": fileID})
	if err != nil || result.DeletedCount == 0 {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete file"})
	}
	return c.JSON(fiber.Map{"message": "File deleted successfully"})
}
