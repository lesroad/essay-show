package class

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Class struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name        string             `bson:"name" json:"name"`
	Description string             `bson:"description" json:"description"`
	InviteCode  string             `bson:"invite_code" json:"inviteCode"`
	CreatorID   string             `bson:"creator_id" json:"creatorId"`
	MemberCount int64              `bson:"member_count" json:"memberCount"`
	CreateTime  time.Time          `bson:"create_time" json:"createTime"`
	UpdateTime  time.Time          `bson:"update_time" json:"updateTime"`
	DeleteTime  time.Time          `bson:"delete_time,omitempty" json:"deleteTime"`
}
