package feedback

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Feedback struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserId     string             `bson:"user_id" json:"userId"`         // 提交反馈的用户ID
	Type       int64              `bson:"type" json:"type"`              // 反馈类型（如：建议、错误报告、功能请求等）
	Content    string             `bson:"content" json:"content"`        // 反馈内容
	Status     int                `bson:"status" json:"status"`          // 处理状态（如：未处理、处理中、已处理）
	Images     []string           `bson:"images" json:"images"`          // 用户上传的图片URL列表（可选）
	CreateTime time.Time          `bson:"create_time" json:"createTime"` // 创建时间
	UpdateTime time.Time          `bson:"update_time" json:"updateTime"` // 更新时间
}
