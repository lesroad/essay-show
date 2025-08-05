package user

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type User struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Username   string             `bson:"username" json:"username"`
	Phone      string             `bson:"phone" json:"phone"`
	Count      int64              `bson:"count" json:"count"` // 剩余可用批改次数
	Status     int                `bson:"status" json:"status"`
	School     string             `bson:"school" json:"school"`
	Grade      int64              `bson:"grade" json:"grade"` // 默认0，从一开始依次递增
	Role       string             `bson:"role" json:"role"`   // 用户角色：student/teacher
	CreateTime time.Time          `bson:"create_time,omitempty" json:"createTime"`
	UpdateTime time.Time          `bson:"update_time,omitempty" json:"updateTime"`
	DeleteTime time.Time          `bson:"delete_time,omitempty" json:"deleteTime"`
}
