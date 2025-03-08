package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// User model for MongoDB
type User struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	Email     string             `bson:"email" json:"email" validate:"required,email"`
	Password  string             `bson:"password,omitempty" json:"-"` // Never expose password in JSON
	Role      string             `bson:"role" json:"role"` // "user" or "admin"
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
}

