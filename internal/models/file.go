package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type File struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Filename      string             `bson:"filename" json:"filename"`
	URL           string             `bson:"url" json:"url"`
	Owner         string             `bson:"owner" json:"owner"`
	ExpiresAt     time.Time          `bson:"expires_at" json:"expires_at"`
	CreatedAt     time.Time          `bson:"created_at" json:"created_at"`
	DownloadToken string             `bson:"download_token,omitempty" json:"-"`
	TokenType     string             `bson:"token_type,omitempty" json:"token_type"` // "one-time" or "time-limited"
	TokenExpires  time.Time          `bson:"token_expires,omitempty" json:"token_expires"`
}

