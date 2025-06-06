package service

import (
	"context"
	"encoding/json"
	"essay-show/biz/adaptor"
	"essay-show/biz/application/dto/essay/show"
	"essay-show/biz/infrastructure/consts"
	"essay-show/biz/infrastructure/mapper/log"
	"essay-show/biz/infrastructure/mapper/user"
	"essay-show/biz/infrastructure/redis"
	"essay-show/biz/infrastructure/util"
	logx "essay-show/biz/infrastructure/util/log"
	"time"

	"github.com/google/wire"
	"github.com/jinzhu/copier"
)

type IEssayService interface {
	EssayEvaluate(ctx context.Context, req *show.EssayEvaluateReq) (resp *show.EssayEvaluateResp, err error)
	GetEvaluateLogs(ctx context.Context, req *show.GetEssayEvaluateLogsReq) (resp *show.GetEssayEvaluateLogsResp, err error)
	LikeEvaluate(ctx context.Context, req *show.LikeEvaluateReq) (resp *show.Response, err error)
}

type EssayService struct {
	LogMapper  *log.MongoMapper
	UserMapper *user.MongoMapper
}

var EssayServiceSet = wire.NewSet(
	wire.Struct(new(EssayService), "*"),
	wire.Bind(new(IEssayService), new(*EssayService)),
)

// EssayEvaluate 根据标题和作文调用批改中台进行批改
func (s *EssayService) EssayEvaluate(ctx context.Context, req *show.EssayEvaluateReq) (*show.EssayEvaluateResp, error) {
	// TODO 应该实现一个用户同时只能调用一次批改

	// 获取登录状态信息
	meta := adaptor.ExtractUserMeta(ctx)
	if meta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}

	// 判断用户是否存在 (meta在不同应用间是互通的, 而次数等由小程序单独管理)
	u, err := s.UserMapper.FindOne(ctx, meta.GetUserId())
	if err != nil {
		return nil, consts.ErrNotFound
	}

	// 剩余次数不足
	if u.Count <= 0 {
		return nil, consts.ErrInSufficientCount
	}

	// 获取锁
	key := "evaluate" + meta.GetUserId()
	lock := redis.NewEvaMutex(ctx, key, 20, 40)
	if err = lock.Lock(); err != nil {
		return nil, consts.ErrOneCall
	}

	// 调用essay-stateless批改作文
	client := util.GetHttpClient()
	_resp, err := client.BetaEvaluate(req.Title, req.Text, req.Grade, req.EssayType)
	if err != nil { // 调用call失败
		return nil, consts.ErrCall
	}

	// 释放锁, 释放锁失败或锁超时不应该记录这一次批改
	if err = lock.Unlock(); err != nil || lock.Expired() {
		return nil, nil
	}

	// 获取批改的结果
	code := int64(_resp["code"].(float64))
	msg := _resp["msg"].(string)
	bytes, err := json.Marshal(_resp)
	if err != nil {
		return nil, err
	}
	result := string(bytes)

	// 构造日志
	l := &log.Log{
		UserId:     meta.GetUserId(),
		Ocr:        req.Ocr,
		Response:   result,
		Status:     int(code),
		CreateTime: time.Now(),
	}
	if req.Grade != nil {
		l.Grade = *req.Grade
	}

	// 批改失败，记录对应的情况
	if code != 0 {
		logx.Error("批改失败 err: %v", err)
		// 存入错误信息，用于后续分析问题 TODO: 后续可能考虑这里通过定时任务存档，并从数据库中删除
		l.Response = err.Error()
		err = s.LogMapper.InsertErr(ctx, l)
		return nil, consts.ErrCall
	}

	resp := &show.EssayEvaluateResp{
		Code:     code,
		Msg:      msg,
		Response: result,
		Id:       l.ID.Hex(),
	}

	// 扣除用户剩余次数
	err = s.UserMapper.UpdateCount(ctx, meta.GetUserId(), -1)
	if err != nil {
		return nil, err //  扣除失败用户不应该拿到结果
	}

	// 存入正确批改结果
	err = s.LogMapper.Insert(ctx, l)
	if err != nil {
		// 记录插入失败应该也要获得结果，因为剩余次数已经成功扣除。 TODO: 需要一个托底逻辑，考虑使用事务
		logx.Error("log insert failed %v", err)
	}
	return resp, nil
}

// GetEvaluateLogs 分页查找获取正常的批改记录
func (s *EssayService) GetEvaluateLogs(ctx context.Context, req *show.GetEssayEvaluateLogsReq) (resp *show.GetEssayEvaluateLogsResp, err error) {
	// 获取用户信息
	meta := adaptor.ExtractUserMeta(ctx)
	if meta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}

	// 分页查询
	data, total, err := s.LogMapper.FindMany(ctx, meta.GetUserId(), req.PaginationOptions)
	if err != nil {
		return nil, err
	}
	var logs []*show.Log
	// 类型转换
	for _, val := range data {
		l := &show.Log{}
		err = copier.Copy(l, val)
		if err != nil {
			return nil, err
		}
		l.Id = val.ID.Hex()
		l.CreateTime = val.CreateTime.Unix()
		logs = append(logs, l)
	}

	return &show.GetEssayEvaluateLogsResp{
		Total: total,
		Logs:  logs,
	}, nil
}

// LikeEvaluate 点赞或点踩一次批改
func (s *EssayService) LikeEvaluate(ctx context.Context, req *show.LikeEvaluateReq) (resp *show.Response, err error) {
	// 查询批改记录
	l, err := s.LogMapper.FindOne(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	// 更新点赞状态
	l.Like = req.Like
	err = s.LogMapper.Update(ctx, l)
	if err != nil {
		logx.Error(err.Error())
		return util.Fail(999, "标记失败"), nil
	}
	return util.Succeed("标记成功")
}
