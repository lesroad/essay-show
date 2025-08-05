package service

import (
	"context"
	"errors"
	"essay-show/biz/adaptor"
	"essay-show/biz/application/dto/essay/show"
	"essay-show/biz/infrastructure/config"
	"essay-show/biz/infrastructure/consts"
	"essay-show/biz/infrastructure/repository/user"
	"essay-show/biz/infrastructure/util"
	"essay-show/biz/infrastructure/util/log"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/google/wire"
)

type IStsService interface {
	ApplySignedUrl(ctx context.Context, req *show.ApplySignedUrlReq) (*show.ApplySignedUrlResp, error)
	OCR(ctx context.Context, req *show.OCRReq) (*show.OCRResp, error)
	SendVerifyCode(ctx context.Context, req *show.SendVerifyCodeReq) (*show.Response, error)
}

type StsService struct {
	UserMapper *user.MongoMapper
}

var StsServiceSet = wire.NewSet(
	wire.Struct(new(StsService), "*"),
	wire.Bind(new(IStsService), new(*StsService)),
)

// ApplySignedUrl 向cos申请加签url
func (s *StsService) ApplySignedUrl(ctx context.Context, req *show.ApplySignedUrlReq) (*show.ApplySignedUrlResp, error) {
	// 获取用户信息
	aUser := adaptor.ExtractUserMeta(ctx)
	if aUser.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}
	// 构造响应
	resp := new(show.ApplySignedUrlResp)
	// 获取cos状态
	userId := aUser.GetUserId()
	client := util.GetHttpClient()
	data, err := client.GenCosSts(ctx, fmt.Sprintf("essays_%s/%s/*", config.GetConfig().State, userId))
	if err != nil {
		return nil, err
	}
	if data["code"].(float64) != 0 {
		return nil, errors.New(data["message"].(string))
	}
	data = data["data"].(map[string]any)

	// 生成加签url
	resp.SessionToken = data["sessionToken"].(string)
	if req.Prefix != nil {
		*req.Prefix += "/"
	}

	data2, err := client.GenSignedUrl(ctx,
		data["secretId"].(string),
		data["secretKey"].(string),
		http.MethodPut,
		fmt.Sprintf("essays_%s/%s/%s%s%s", config.GetConfig().State, userId, req.GetPrefix(), uuid.New().String(), req.GetSuffix()),
	)
	if err != nil || data2["code"].(float64) != 0 {
		return nil, err
	}
	data2 = data2["data"].(map[string]any)

	// 返回响应
	resp.Url = data2["signedUrl"].(string)
	return resp, nil
}

func (s *StsService) OCR(ctx context.Context, req *show.OCRReq) (*show.OCRResp, error) {
	aUser := adaptor.ExtractUserMeta(ctx)
	if aUser.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}

	// 查询用户信息
	u, err := s.UserMapper.FindOne(ctx, aUser.GetUserId())
	if err != nil {
		return nil, consts.ErrNotFound
	}

	// 检查剩余次数
	if u.Count <= 0 {
		return nil, consts.ErrInSufficientCount
	}

	images := req.Ocr
	left := ""
	if req.LeftType != nil {
		left = *req.LeftType
	}

	client := util.GetHttpClient()
	resp, err := client.TitleUrlOCR(ctx, images, left)
	if err != nil {
		return nil, err
	}
	if resp["code"].(float64) != 0 {
		return nil, consts.ErrOCR
	}
	data := resp["data"].(map[string]any)
	if data == nil {
		return nil, consts.ErrOCR
	}

	return &show.OCRResp{Title: data["title"].(string), Text: data["content"].(string)}, nil
}

// SendVerifyCode 发送验证码
func (s *StsService) SendVerifyCode(ctx context.Context, req *show.SendVerifyCodeReq) (*show.Response, error) {
	// 查找用户
	aUser, err := s.UserMapper.FindOneByPhone(ctx, req.AuthId)

	if req.Type == 1 { // 登录验证码
		// 查找数据库判断手机号是否注册过
		if errors.Is(err, consts.ErrNotFound) || aUser == nil { // 未找到，说明没有注册
			return nil, consts.ErrNotSignUp
		} else if err != nil {
			return nil, consts.ErrSend
		}
	} else { // 注册验证码
		if err == nil && aUser != nil {
			return nil, consts.ErrRepeatedSignUp
		} else if err != nil && !errors.Is(err, consts.ErrNotFound) {
			return nil, consts.ErrSignUp
		}
	}

	// 通过中台发送验证码
	httpClient := util.GetHttpClient()
	ret, err := httpClient.SendVerifyCode(ctx, req.AuthType, req.AuthId)
	if err != nil || ret["code"].(float64) != 0 {
		log.Error("发送验证码失败:%v, ret:%v", err, ret)
		return nil, consts.ErrSend
	}

	return util.Succeed("发送验证码成功，请注意查收")
}
