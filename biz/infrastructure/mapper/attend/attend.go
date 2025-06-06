package attend

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

// Attend 记录用户每日的签到情况
type Attend struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"` // uid
	UserId    string             `bson:"user_id"`       // 记录的用户Id
	Timestamp time.Time          `bson:"timestamp"`     // 签到的时间
}
