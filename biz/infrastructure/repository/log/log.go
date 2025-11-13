package log

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Log struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserId     string             `bson:"user_id" json:"user_id"`
	Grade      int64              `bson:"grade" json:"grade"`
	Ocr        []string           `bson:"ocr" json:"ocr"`
	Response   string             `bson:"response" json:"response"`
	Like       int64              `bson:"like" json:"like"`
	Status     int                `bson:"status" json:"status"` // 0: 正常, 1: 已修改
	CreateTime time.Time          `bson:"create_time,omitempty" json:"createTime"`
}
