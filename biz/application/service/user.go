package service

import (
	"context"
	"errors"
	"essay-show/biz/adaptor"
	"essay-show/biz/application/dto/essay/show"
	"essay-show/biz/application/dto/essay/sts"
	"essay-show/biz/infrastructure/consts"
	"essay-show/biz/infrastructure/repository/attend"
	"essay-show/biz/infrastructure/repository/invitation"
	"essay-show/biz/infrastructure/repository/user"
	"essay-show/biz/infrastructure/util"
	"essay-show/biz/infrastructure/util/log"
	"time"

	"github.com/google/wire"
	"github.com/mitchellh/mapstructure"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type IUserService interface {
	SignIn(ctx context.Context, req *show.SignInReq) (*show.SignInResp, error)
	BindAuth(ctx context.Context, req *show.BindAuthReq) (*show.BindAuthResp, error)
	GetUserInfo(ctx context.Context, req *show.GetUserInfoReq) (*show.GetUserInfoResp, error)
	UpdateUserInfo(ctx context.Context, req *show.UpdateUserInfoReq) (*show.Response, error)
	DailyAttend(ctx context.Context, req *show.DailyAttendReq) (*show.Response, error)
	GetDailyAttend(ctx context.Context, req *show.GetDailyAttendReq) (*show.GetDailyAttendResp, error)
	FillInvitationCode(ctx context.Context, req *show.FillInvitationCodeReq) (*show.Response, error)
	GetInvitationCode(ctx context.Context, req *show.GetInvitationCodeReq) (*show.GetInvitationCodeResp, error)
}
type UserService struct {
	UserMapper   *user.MongoMapper
	AttendMapper *attend.MongoMapper
	CodeMapper   *invitation.CodeMongoMapper
	LogMapper    *invitation.LogMongoMapper
}

var UserServiceSet = wire.NewSet(
	wire.Struct(new(UserService), "*"),
	wire.Bind(new(IUserService), new(*UserService)),
)

// SignIn 登录用户
func (s *UserService) SignIn(ctx context.Context, req *show.SignInReq) (*show.SignInResp, error) {
	var u *user.User
	var err error

	httpClient := util.GetHttpClient()
	signInResponse, err := httpClient.SignIn(ctx, req.AuthType, req.AuthId, req.VerifyCode, req.Password)
	if err != nil || signInResponse["code"].(float64) != 0 {
		return nil, consts.ErrSignIn
	}
	resp := new(sts.SignInResp)
	if dataMap, ok := signInResponse["data"].(map[string]any); ok {
		if err := mapstructure.Decode(dataMap, resp); err != nil {
			return nil, consts.ErrSignIn
		}
	} else {
		return nil, consts.ErrSignIn
	}

	accessToken, accessExpire, err := adaptor.GenerateJwtToken(resp)
	if err != nil {
		return nil, consts.ErrSignIn
	}

	userId := resp.UserId

	u, err = s.UserMapper.FindOne(ctx, userId)
	if errors.Is(err, consts.ErrNotFound) || u == nil {
		// 注册流程
		oid, err2 := primitive.ObjectIDFromHex(userId)
		if err2 != nil {
			return nil, err2
		}
		now := time.Now()
		u = &user.User{
			ID:         oid,
			Username:   "未设置用户名",
			Count:      consts.DefaultCount,
			Status:     0,
			Role:       consts.RoleStudent,
			CreateTime: now,
			UpdateTime: now,
		}
		if (req.AuthType == "wechat-phone" || req.AuthType == "web-phone") && resp.Options != nil {
			u.Phone = *resp.Options
		} else if req.AuthType == "phone" {
			u.Phone = req.AuthId
		}

		err = s.UserMapper.Insert(ctx, u)
		if err != nil {
			return nil, consts.ErrSignUp
		}
	} else if err != nil {
		return nil, consts.ErrSignIn
	}

	return &show.SignInResp{
		Id:           userId,
		AccessToken:  accessToken,
		AccessExpire: accessExpire,
		Name:         u.Username,
	}, nil
}

func (s *UserService) BindAuth(ctx context.Context, req *show.BindAuthReq) (*show.BindAuthResp, error) {
	// 获取用户id
	userMeta := adaptor.ExtractUserMeta(ctx)
	if userMeta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}

	// 在中台绑定授权
	httpClient := util.GetHttpClient()
	bindAuthResponse, err := httpClient.BindAuth(ctx, req.AuthType, req.AuthId, req.VerifyCode, userMeta.GetUserId())
	if err != nil || bindAuthResponse["code"].(float64) != 0 {
		return nil, consts.ErrBindAuth
	}
	data := bindAuthResponse["data"].(map[string]any)

	u, err := s.UserMapper.FindOne(ctx, userMeta.GetUserId())
	if err != nil {
		return nil, consts.ErrNotFound
	}
	switch req.AuthType {
	case "wechat-phone":
		u.Phone = data["options"].(string)
	case "wechat-openid":
	default:
		return nil, consts.ErrBindAuth
	}

	err = s.UserMapper.Update(ctx, u)
	if err != nil {
		return nil, consts.ErrBindAuth
	}

	return &show.BindAuthResp{
		Code: 0,
		Msg:  "绑定成功",
	}, nil
}

// GetUserInfo 获取用户信息
func (s *UserService) GetUserInfo(ctx context.Context, req *show.GetUserInfoReq) (*show.GetUserInfoResp, error) {
	// 用户信息
	meta := adaptor.ExtractUserMeta(ctx)
	if meta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}

	// 查询用户
	u, err := s.UserMapper.FindOne(ctx, meta.GetUserId())
	if err != nil {
		return &show.GetUserInfoResp{
			Code:    -1,
			Msg:     "查询用户信息失败，请先登录或重试",
			Payload: nil,
		}, nil
	}

	// 确定用户角色
	role := show.UserRole_STUDENT // 默认为学生
	if u.Role == consts.RoleTeacher {
		role = show.UserRole_TEACHER
	}

	return &show.GetUserInfoResp{
		Code: 0,
		Msg:  "查询成功",
		Payload: &show.GetUserInfoResp_Payload{
			Name:  u.Username,
			Count: u.Count,
			Phone: u.Phone,
			Role:  role,
		},
	}, nil
}

// UpdateUserInfo 更新用户信息
func (s *UserService) UpdateUserInfo(ctx context.Context, req *show.UpdateUserInfoReq) (*show.Response, error) {
	userMeta := adaptor.ExtractUserMeta(ctx)
	if userMeta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}

	u, err := s.UserMapper.FindOne(ctx, userMeta.GetUserId())
	if err != nil {
		return nil, consts.ErrNotFound
	}

	if req.Name != nil {
		u.Username = *req.Name
	}
	if req.School != nil {
		u.School = *req.School
	}
	if req.Grade != nil {
		u.Grade = *req.Grade
	}

	if req.Role != nil {
		if *req.Role == show.UserRole_TEACHER {
			u.Role = consts.RoleTeacher
		} else if *req.Role == show.UserRole_STUDENT {
			u.Role = consts.RoleStudent
		}
	}

	err = s.UserMapper.Update(ctx, u)
	if err != nil {
		return nil, consts.ErrUpdate
	}

	return &show.Response{
		Code: 0,
		Msg:  "更新成功",
	}, nil
}

func (s *UserService) DailyAttend(ctx context.Context, req *show.DailyAttendReq) (*show.Response, error) {
	// 用户信息
	meta := adaptor.ExtractUserMeta(ctx)
	if meta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}

	// 查询最近的attend记录
	a, err := s.findAttend(ctx, meta.GetUserId())
	if err != nil && !errors.Is(err, consts.ErrNotFound) {
		return nil, consts.ErrDailyAttend
	}

	// 今日有签到记录且不是第一次签到
	if a != nil && time.Unix(a.Timestamp.Unix(), 0).Day() == time.Now().Day() && !a.Timestamp.IsZero() {
		return nil, consts.ErrRepeatDailyAttend
	}

	// 插入新的签到记录
	_a := &attend.Attend{
		ID:        primitive.NewObjectID(),
		UserId:    meta.GetUserId(),
		Timestamp: time.Now(),
	}
	err = s.AttendMapper.Insert(ctx, _a)
	if err != nil {
		return nil, consts.ErrDailyAttend
	}

	// 增加次数
	err = s.UserMapper.UpdateCount(ctx, meta.GetUserId(), consts.AttendReward)
	if err != nil {
		return nil, consts.ErrDailyAttend
	}

	return util.Succeed("签到成功")
}

func (s *UserService) GetDailyAttend(ctx context.Context, req *show.GetDailyAttendReq) (*show.GetDailyAttendResp, error) {
	resp := &show.GetDailyAttendResp{
		Code:   0,
		Msg:    "success",
		Attend: 0,
	}

	// 用户信息
	meta := adaptor.ExtractUserMeta(ctx)
	if meta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}

	// 获取最新的, 确定今天的更新状态
	a, err := s.findAttend(ctx, meta.GetUserId())
	if err != nil {
		log.Error("获取签到记录失败, err:%v", err.Error())
		return nil, consts.ErrNotFound
	}
	if !a.Timestamp.IsZero() && time.Unix(a.Timestamp.Unix(), 0).Day() == time.Now().Day() {
		resp.Attend = 1
	}

	// 获取所有的指定年月的所有签到记录
	data, total, err := s.AttendMapper.FindByYearAndMonth(ctx, meta.GetUserId(), int(req.Year), int(req.Month))
	if err != nil {
		log.Error("获取签到记录失败, err:%v", err.Error())
		return nil, consts.ErrNotFound
	}

	dtos := make([]int64, 0, len(data))
	for _, d := range data {
		dtos = append(dtos, int64(d.Timestamp.Day()))
	}
	resp.History = dtos
	resp.Total = total

	return resp, nil
}

func (s *UserService) FillInvitationCode(ctx context.Context, req *show.FillInvitationCodeReq) (*show.Response, error) {
	// 用户信息
	userMeta := adaptor.ExtractUserMeta(ctx)
	if userMeta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}

	// 获取邀请码对应邀请者
	c, err := s.CodeMapper.FindOneByCode(ctx, req.InvitationCode)
	if err != nil {
		return nil, consts.ErrNotFound
	}

	inviter := c.UserId
	invitee := userMeta.GetUserId()

	if invitee == inviter {
		return nil, consts.ErrInvitation
	}

	// 尝试获取邀请记录
	l, err := s.LogMapper.FindOneByInvitee(ctx, invitee)
	if err == nil && l != nil {
		// 已填过邀请码
		return nil, consts.ErrRepeatInvitation
	} else if !errors.Is(err, consts.ErrNotFound) {
		// 异常
		return nil, err
	}

	// 插入邀请记录
	err = s.LogMapper.Insert(ctx, inviter, invitee)
	if err != nil {
		return nil, consts.ErrInvitation
	}

	err = s.UserMapper.UpdateCount(ctx, inviter, consts.InvitationReward)
	if err != nil {
		return nil, consts.ErrUpdate
	}
	return util.Succeed("success")
}

func (s *UserService) GetInvitationCode(ctx context.Context, req *show.GetInvitationCodeReq) (*show.GetInvitationCodeResp, error) {
	// 用户信息
	userMeta := adaptor.ExtractUserMeta(ctx)
	if userMeta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}

	c, err := s.CodeMapper.FindOneByUserId(ctx, userMeta.GetUserId())
	if errors.Is(err, consts.ErrNotFound) {
		c, err = s.CodeMapper.Insert(ctx, userMeta.GetUserId())
		if err != nil {
			log.Error("获取邀请码失败, err:%v", err.Error())
			return nil, consts.ErrCall
		}
	} else if err != nil {
		log.Error("获取邀请码失败, err:%v", err.Error())
		return nil, consts.ErrCall
	}

	return &show.GetInvitationCodeResp{
		Code:           0,
		Msg:            "success",
		InvitationCode: c.Code,
	}, nil
}

func (s *UserService) findAttend(ctx context.Context, userId string) (*attend.Attend, error) {
	a, err := s.AttendMapper.FindLatestOneByUserId(ctx, userId)
	return a, err
}
