package user

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type User struct {
	ID       primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Username string             `bson:"username" json:"username"`
	Phone    string             `bson:"phone" json:"phone"`
	Count    int64              `bson:"count" json:"count"` // 剩余可用批改次数
	Status   int                `bson:"status" json:"status"`
	School   string             `bson:"school" json:"school"`
	Grade    int64              `bson:"grade" json:"grade"` // 默认0，从一开始依次递增
	Role     string             `bson:"role" json:"role"`   // 用户角色：student/teacher/admin
	// MBA 记忆摘要，key 为 essay_type（如 "199_lunxiao"），value 为上次批改后更新的 memory_summary
	MbaMemory map[string]string `bson:"mba_memory,omitempty" json:"mbaMemory"`
	// VipExpireTime 是会员是否生效的唯一来源：会员为一次性购买时长（xpay 虚拟支付），无自动续费，
	// 过期后不做任何状态迁移，是否为 VIP 始终由 IsVipActive 基于该字段实时判断。
	VipExpireTime time.Time `bson:"vip_expire_time,omitempty" json:"vipExpireTime"`
	CreateTime    time.Time `bson:"create_time,omitempty" json:"createTime"`
	UpdateTime    time.Time `bson:"update_time,omitempty" json:"updateTime"`
	DeleteTime    time.Time `bson:"delete_time,omitempty" json:"deleteTime"`
}

func IsVipActive(u *User) bool {
	return u.VipExpireTime.After(time.Now())
}
