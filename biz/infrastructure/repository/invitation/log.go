package invitation

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Log struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	Inviter   string             `bson:"inviter"`
	Invitee   string             `bson:"invitee"`
	Source    *string            `bson:"source,omitempty"`
	Timestamp time.Time          `bson:"timestamp"`
}
