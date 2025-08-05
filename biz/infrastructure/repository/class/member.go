package class

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ClassMember struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ClassID    string             `bson:"class_id" json:"classId"`
	UserID     string             `bson:"user_id" json:"userId"`
	Role       string             `bson:"role" json:"role"` // teacher/student
	JoinTime   time.Time          `bson:"join_time" json:"joinTime"`
	CreateTime time.Time          `bson:"create_time" json:"createTime"`
	UpdateTime time.Time          `bson:"update_time" json:"updateTime"`
}
