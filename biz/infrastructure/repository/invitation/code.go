package invitation

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type Code struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	UserId    string             `bson:"user_id"`
	Code      string             `bson:"code"`
	Timestamp time.Time          `bson:"timestamp"`
}
