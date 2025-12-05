package consts

import "time"

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
	RoleStudent  = "student"
	RoleTeacher  = "teacher"
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

const (
	// 作业状态常量
	StatusNotSubmission = -1
	StatusInitialized   = 0 // 初始化
	StatusGrading       = 1 // 批改中
	StatusCompleted     = 2 // 批改完成
	StatusModified      = 3 // 已人工修改
	StatusFailed        = 7 // 批改失败

	// 定时器配置常量
	TimerInterval   = 30 * time.Second // 扫描间隔
	TimeoutDuration = 20 * time.Minute // 超时时间

	InvitationTemplateId = "KglmTXE65kiACeTM85kwpA2oO9SU0urRGBJTo4gH9O0"
	InvitationJumpPage = "pages/tabbar/profile"
)

const (
	AuthTypeEmail           = "email"
	AuthTypePhone           = "phone"
	AuthTypeAccountPassword = "account-password"
	AuthTypeWechatOpenId    = "wechat-openid"
	AuthTypeWechatUnionId   = "wechat-unionid"
	AuthTypeWechatPhone     = "wechat-phone"
	AuthTypeWebPhone        = "web-phone"
)
