package services

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/arzan03/SecureShare/internal/db"
	"github.com/arzan03/SecureShare/internal/models"
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"
)

var jwtSecret = os.Getenv("JWT_SECRET")


// HashPassword hashes a password using bcrypt
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

// VerifyPassword compares a plain password with a hashed password
func VerifyPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// GenerateJWT generates a JWT token with user ID and role
func GenerateJWT(userID string, role string) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"role":    role,
		"exp":     time.Now().Add(time.Hour * 4).Unix(), // Token expires in 4 hours
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(jwtSecret))
}

// RegisterUser registers a new user with role validation
func RegisterUser(email, password, role string) (models.User, error) {
	collection := db.GetCollection("secure_files", "users")

	// Check if user already exists
	var existingUser models.User
	err := collection.FindOne(context.TODO(), bson.M{"email": email}).Decode(&existingUser)
	if err == nil {
		return models.User{}, errors.New("email already in use")
	}

	
	// Hash password
	hashedPassword, err := HashPassword(password)
	if err != nil {
		return models.User{}, err
	}

	// Create new user
	user := models.User{
		ID:        primitive.NewObjectID(),
		Email:     email,
		Password:  hashedPassword,
		Role:      "user",
		CreatedAt: time.Now(),
	}
	_, err = collection.InsertOne(context.TODO(), user)
	return user, err
}

// LoginUser authenticates a user and returns a JWT with role info
func LoginUser(email, password string) (string, error) {
	collection := db.GetCollection("secure_files", "users")

	var user models.User
	err := collection.FindOne(context.TODO(), bson.M{"email": email}).Decode(&user)
	if err != nil {
		return "",  errors.New("invalid credentials")
	}

	// Verify password
	if !VerifyPassword(password, user.Password) {
		return "", errors.New("invalid credentials")
	}

	// Generate JWT including role
	token, err := GenerateJWT(user.ID.Hex(), user.Role)
	if err != nil {
		return "", err
	}

	return token, nil
}
