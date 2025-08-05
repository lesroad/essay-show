package homework

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Homework struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Title       string             `bson:"title" json:"title"`
	Description string             `bson:"description" json:"description"`
	ClassIDs    []string           `bson:"class_ids" json:"classIds"`
	Deadline    time.Time          `bson:"deadline" json:"deadline"`
	EssayType   string             `bson:"essay_type" json:"essayType"`
	CreatorID   string             `bson:"creator_id" json:"creatorId"`
	CreatorName string             `bson:"creator_name" json:"creatorName"`
	CreateTime  time.Time          `bson:"create_time" json:"createTime"`
	UpdateTime  time.Time          `bson:"update_time" json:"updateTime"`
	DeleteTime  time.Time          `bson:"delete_time,omitempty" json:"deleteTime"`
}
