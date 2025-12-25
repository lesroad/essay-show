package consts

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Errno struct {
	err  error
	code codes.Code
}

// GRPCStatus 实现 GRPCStatus 方法
func (en *Errno) GRPCStatus() *status.Status {
	return status.New(en.code, en.err.Error())
}

// 实现 Error 方法
func (en *Errno) Error() string {
	return en.err.Error()
}

// NewErrno 创建自定义错误
func NewErrno(code codes.Code, err error) *Errno {
	return &Errno{
		err:  err,
		code: code,
	}
}

// 定义常量错误
var (
	ErrForbidden                   = NewErrno(codes.PermissionDenied, errors.New("forbidden"))
	ErrNotAuthentication           = NewErrno(codes.Code(1000), errors.New("not authentication"))
	ErrSignUp                      = NewErrno(codes.Code(1001), errors.New("注册失败，请重试"))
	ErrSignIn                      = NewErrno(codes.Code(1002), errors.New("登录失败，请先注册或重试"))
	ErrInSufficientCount           = NewErrno(codes.Code(1003), errors.New("剩余调用次数不足，请充值或联系管理员添加"))
	ErrRepeatedSignUp              = NewErrno(codes.Code(1004), errors.New("该手机号已注册"))
	ErrOCR                         = NewErrno(codes.Code(1005), errors.New("OCR识别失败，请重试"))
	ErrNotSignUp                   = NewErrno(codes.Code(1006), errors.New("请确认手机号已注册"))
	ErrSend                        = NewErrno(codes.Code(1007), errors.New("发送验证码失败，请重试"))
	ErrVerifyCode                  = NewErrno(codes.Code(1008), errors.New("验证码错误"))
	ErrDailyAttend                 = NewErrno(codes.Code(1009), errors.New("签到失败"))
	ErrRepeatDailyAttend           = NewErrno(codes.Code(1010), errors.New("一天只能签到一次"))
	ErrRepeatInvitation            = NewErrno(codes.Code(1011), errors.New("已填写过邀请码"))
	ErrInvitation                  = NewErrno(codes.Code(1011), errors.New("填写邀请码失败，请重试"))
	ErrCreateExercise              = NewErrno(codes.Code(1012), errors.New("创建练习失败"))
	ErrDoExercise                  = NewErrno(codes.Code(1013), errors.New("提交练习失败"))
	ErrBindAuth                    = NewErrno(codes.Code(1014), errors.New("绑定授权失败"))
	ErrCreateClass                 = NewErrno(codes.Code(1015), errors.New("创建班级失败"))
	ErrGetClassList                = NewErrno(codes.Code(1016), errors.New("获取班级列表失败"))
	ErrJoinClass                   = NewErrno(codes.Code(1017), errors.New("加入班级失败"))
	ErrGetClassMembers             = NewErrno(codes.Code(1018), errors.New("获取班级成员失败"))
	ErrCreateHomework              = NewErrno(codes.Code(1019), errors.New("创建作业失败"))
	ErrGetHomeworkList             = NewErrno(codes.Code(1020), errors.New("获取作业列表失败"))
	ErrSubmitHomework              = NewErrno(codes.Code(1021), errors.New("提交作业失败"))
	ErrGradeHomework               = NewErrno(codes.Code(1022), errors.New("批改作业失败"))
	ErrGetSubmission               = NewErrno(codes.Code(1023), errors.New("获取提交详情失败"))
	ErrGetHomework                 = NewErrno(codes.Code(1024), errors.New("获取作业详情失败"))
	ErrHomeworkNotGrade            = NewErrno(codes.Code(1024), errors.New("作业未批改完成"))
	ErrNotClassMember              = NewErrno(codes.Code(1025), errors.New("用户不是班级成员"))
	ErrInvalidScore                = NewErrno(codes.Code(1026), errors.New("分数不能为负数"))
	ErrScoreSumMismatch            = NewErrno(codes.Code(1027), errors.New("自定义评分总和必须等于总分"))
	ErrIncompleteScoreDistribution = NewErrno(codes.Code(1028), errors.New("自定义评分必须包含内容、表达、结构(或发展)三项"))
	ErrInvalidScoreDistribution    = NewErrno(codes.Code(1029), errors.New("结构和发展不能同时设置"))
	ErrSendWechatMessage           = NewErrno(codes.Code(1030), errors.New("发送微信消息失败"))
	ErrNoCompletedSubmissions      = NewErrno(codes.Code(1031), errors.New("没有已批改完成的提交"))
)

// ErrInvalidParams 调用时错误
var (
	ErrInvalidParams = NewErrno(codes.InvalidArgument, errors.New("参数错误"))
	ErrCall          = NewErrno(codes.Unknown, errors.New("调用接口失败，请重试"))
	ErrOneCall       = NewErrno(codes.Code(3001), errors.New("同一时刻仅可以批改一篇作文, 请等待上一篇作文批改结束"))
)

// 数据库相关错误
var (
	ErrNotFound        = NewErrno(codes.NotFound, errors.New("not found"))
	ErrInvalidObjectId = NewErrno(codes.InvalidArgument, errors.New("无效的id "))
	ErrUpdate          = NewErrno(codes.Code(2001), errors.New("更新失败"))
)
