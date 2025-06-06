package consts

var PageSize int64 = 10

// 数据库相关
const (
	ID           = "_id"
	UserID       = "user_id"
	Status       = "status"
	CreateTime   = "create_time"
	DeleteStatus = 3
	EffectStatus = 0
	Phone        = "phone"
	Timestamp    = "timestamp"
	LogId        = "log_id"
	NotEqual     = "$ne"
)

// http
const (
	Post            = "POST"
	ContentTypeJson = "application/json"
	CharSetUTF8     = "UTF-8"
)

// 默认值
const (
	DefaultCount     = 30
	AppId            = 14
	Like             = 1
	DisLike          = -1
	InvitationReward = 10
	AttendReward     = 1
)
