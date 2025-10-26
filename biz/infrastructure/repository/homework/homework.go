package homework

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Homework struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Subject     int64              `bson:"subject" json:"subject"`
	Title       string             `bson:"title" json:"title"`
	Description string             `bson:"description" json:"description"`
	ClassID     string             `bson:"class_id" json:"classId"`
	Grade       int64              `bson:"grade" json:"grade"`
	TotalScore  int64              `bson:"total_score" json:"totalScore"`
	EssayType   string             `bson:"essay_type" json:"essayType"`
	CreatorID   string             `bson:"creator_id" json:"creatorId"`
	CreateTime  time.Time          `bson:"create_time" json:"createTime"`
	UpdateTime  time.Time          `bson:"update_time" json:"updateTime"`
	DeleteTime  time.Time          `bson:"delete_time,omitempty" json:"deleteTime"`
}
