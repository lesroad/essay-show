package invitation

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type Log struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	Inviter   string             `bson:"inviter"`
	Invitee   string             `bson:"invitee"`
	Timestamp time.Time          `bson:"timestamp"`
}
