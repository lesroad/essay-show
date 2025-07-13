package service

import (
	"context"
	"encoding/json"
	"essay-show/biz/adaptor"
	"essay-show/biz/application/dto/essay/show"
	"essay-show/biz/application/dto/essay/stateless"
	"essay-show/biz/infrastructure/cache"
	"essay-show/biz/infrastructure/consts"
	"essay-show/biz/infrastructure/lock"
	"essay-show/biz/infrastructure/repository/log"
	"essay-show/biz/infrastructure/repository/user"
	"essay-show/biz/infrastructure/util"
	logx "essay-show/biz/infrastructure/util/log"
	"time"

	"github.com/google/wire"
	"github.com/jinzhu/copier"
)

type IEssayService interface {
	EssayEvaluate(ctx context.Context, req *show.EssayEvaluateReq) (resp *show.EssayEvaluateResp, err error)
	EssayEvaluateStream(ctx context.Context, req *show.EssayEvaluateReq, resultChan chan<- string) error
	GetEvaluateLogs(ctx context.Context, req *show.GetEssayEvaluateLogsReq) (resp *show.GetEssayEvaluateLogsResp, err error)
	LikeEvaluate(ctx context.Context, req *show.LikeEvaluateReq) (resp *show.Response, err error)
	DownloadEvaluate(ctx context.Context, req *show.DownloadEvaluateReq) (resp *show.DownloadEvaluateResp, err error)
}

type EssayService struct {
	LogMapper           *log.MongoMapper
	UserMapper          *user.MongoMapper
	DownloadCacheMapper *cache.DownloadCacheMapper
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

	// 获取锁 - 使用lock包的分布式锁，调整TTL适应复杂批改
	key := "evaluate" + meta.GetUserId()
	distributedLock := lock.NewEvaMutex(ctx, key, 30, 200)
	if err = distributedLock.Lock(); err != nil {
		return nil, consts.ErrOneCall
	}

	// 调用essay-stateless批改作文
	client := util.GetHttpClient()
	_resp, err := client.Evaluate(ctx, req.Title, req.Text, req.Grade, req.EssayType)
	if err != nil {
		logx.Error("call error: %v, req.Text:%s", err, req.Text)
		return nil, consts.ErrCall
	}

	// 释放锁, 释放锁失败或锁超时不应该记录这一次批改
	if err = distributedLock.Unlock(); err != nil || distributedLock.Expired() {
		logx.Error("unlock error: %v, lock expired: %v", err, distributedLock.Expired())
	}

	// 获取批改的结果
	code := int64(_resp["code"].(float64))
	msg := _resp["message"].(string)
	bytes, err := json.Marshal(_resp["data"].(map[string]any))
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

	// 批改失败记录
	if code != 0 {
		logx.Error("essay evaluate failed: %v", err)
		l.Response = msg
		s.LogMapper.InsertErr(ctx, l)
		return nil, consts.ErrCall
	}

	// 批改成功记录
	err = s.LogMapper.Insert(ctx, l)
	if err != nil {
		logx.Error("log insert failed %v", err)
		return nil, consts.ErrCall
	}

	// 扣除用户剩余次数
	err = s.UserMapper.UpdateCount(ctx, meta.GetUserId(), -1)
	if err != nil {
		logx.Error("user count update failed %v", err)
		return nil, consts.ErrCall
	}

	return &show.EssayEvaluateResp{
		Code:     code,
		Msg:      msg,
		Response: result,
		Id:       l.ID.Hex(),
	}, nil
}

// EssayEvaluateStream 流式批改作文
func (s *EssayService) EssayEvaluateStream(ctx context.Context, req *show.EssayEvaluateReq, resultChan chan<- string) error {
	// 同步前置检查 - 与非流式保持一致
	meta := adaptor.ExtractUserMeta(ctx)
	if meta.GetUserId() == "" {
		util.SendStreamMessage(resultChan, util.STError, "用户未认证", nil)
		return consts.ErrNotAuthentication
	}

	// 查询用户信息
	u, err := s.UserMapper.FindOne(ctx, meta.GetUserId())
	if err != nil {
		util.SendStreamMessage(resultChan, util.STError, "用户不存在", nil)
		return consts.ErrNotFound
	}

	// 检查剩余次数
	if u.Count <= 0 {
		util.SendStreamMessage(resultChan, util.STError, "剩余次数不足", nil)
		return consts.ErrInSufficientCount
	}

	// 获取锁 - 调整TTL以适应复杂作文批改时间
	key := "evaluate" + meta.GetUserId()
	distributedLock := lock.NewEvaMutex(ctx, key, 30, 200)
	if err = distributedLock.Lock(); err != nil {
		util.SendStreamMessage(resultChan, util.STError, "当前有批改任务正在进行中", nil)
		return consts.ErrOneCall
	}

	defer func() {
		// 释放锁
		if err = distributedLock.Unlock(); err != nil || distributedLock.Expired() {
			logx.Error("unlock error: %v, lock expired: %v", err, distributedLock.Expired())
		}
	}()

	// 创建内部通道来接收下游结果
	downstreamChan := make(chan string, 100)
	var finalResult string

	// 启动下游调用
	go func() {
		defer close(downstreamChan) // 确保HTTP请求完成后关闭channel，避免主函数永远阻塞
		client := util.GetHttpClient()
		// 使用带context的方法调用下游服务，确保链路追踪信息传递
		client.EvaluateStream(ctx, req.Title, req.Text, req.Grade, req.EssayType, downstreamChan)
	}()

	for jsonMessage := range downstreamChan {
		// 解析下游JSON消息
		var data map[string]interface{}
		if parseErr := json.Unmarshal([]byte(jsonMessage), &data); parseErr != nil {
			logx.Error("解析下游JSON消息失败: %v", parseErr)
			continue
		}
		// 检查消息类型并转发
		if msgType, ok := data["type"].(string); ok {
			switch msgType {
			case "progress":
				util.SendStreamMessage(resultChan, util.STPart, data["message"].(string), data["data"])
			case "complete":
				if result, ok := data["data"].(map[string]interface{}); ok {
					if resultBytes, err := json.Marshal(result); err == nil {
						finalResult = string(resultBytes)
					}
				}
				goto exitLoop
			case "error":
				util.SendStreamMessage(resultChan, util.STError, "下游服务错误", data["data"])
				return consts.ErrCall
			default:
			}
		}
	}

exitLoop:
	if err != nil || len(finalResult) == 0 {
		util.SendStreamMessage(resultChan, util.STError, "批改失败", nil)
		return consts.ErrCall
	}

	l := &log.Log{
		UserId:     meta.GetUserId(),
		Ocr:        req.Ocr,
		Response:   finalResult,
		Status:     0, // 流式批改成功
		CreateTime: time.Now(),
	}
	if req.Grade != nil {
		l.Grade = *req.Grade
	}

	err = s.LogMapper.Insert(ctx, l)
	if err != nil {
		logx.Error("log insert failed %v", err)
		util.SendStreamMessage(resultChan, util.STError, "日志记录失败", nil)
		return consts.ErrCall
	}

	// 扣除用户剩余次数
	err = s.UserMapper.UpdateCount(ctx, meta.GetUserId(), -1)
	if err != nil {
		logx.Error("user count update failed %v", err)
		util.SendStreamMessage(resultChan, util.STError, "用户次数扣减失败", nil)
		return consts.ErrCall
	}

	// 发送最终完成消息
	finalData := &show.EssayEvaluateResp{
		Id:       l.ID.Hex(),
		Code:     0,
		Msg:      "批改完成",
		Response: finalResult,
	}
	util.SendStreamMessage(resultChan, util.STComplete, "批改已完成", finalData)
	return nil
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
		return nil, consts.ErrNotFound
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
		return nil, consts.ErrNotFound
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

// DownloadEvaluate 下载批改结果
func (s *EssayService) DownloadEvaluate(ctx context.Context, req *show.DownloadEvaluateReq) (resp *show.DownloadEvaluateResp, err error) {
	// 获取登录状态信息
	meta := adaptor.ExtractUserMeta(ctx)
	if meta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}

	// 首先尝试从缓存获取
	if cachedResp, err := s.DownloadCacheMapper.Get(ctx, req.Id); err == nil {
		logx.Info("缓存命中，直接返回下载链接, id: %s", req.Id)
		return cachedResp, nil
	}

	// 根据ID查询批改记录
	l, err := s.LogMapper.FindOne(ctx, req.Id)
	if err != nil {
		logx.Error("查询批改记录失败: %v", err)
		return nil, consts.ErrNotFound
	}

	// 验证记录是否属于当前用户
	if l.UserId != meta.GetUserId() {
		return nil, consts.ErrNotFound
	}

	// 解析存储的批改结果到结构体
	var evaluateResult stateless.Evaluate
	if err := json.Unmarshal([]byte(l.Response), &evaluateResult); err != nil {
		logx.Error("解析批改结果失败: %v", err)
		return nil, consts.ErrCall
	}

	// 构造下载请求参数
	downloadData := map[string]interface{}{
		"data":      evaluateResult,
		"user_id":   meta.GetUserId(),
		"watermark": true,
	}

	// 调用下游API生成下载链接
	client := util.GetHttpClient()
	_resp, err := client.EssayPolish(ctx, downloadData)
	if err != nil {
		logx.Error("调用批改结果下载服务失败: %v", err)
		return nil, consts.ErrCall
	}

	// 检查下游响应
	code := int64(_resp["code"].(float64))
	if code != 200 {
		msg := _resp["msg"].(string)
		logx.Error("批改结果下载服务返回错误: %s", msg)
		return nil, consts.ErrCall
	}

	url, urlOk := _resp["signedUrl"].(string)
	sessionToken, tokenOk := _resp["sessionToken"].(string)

	if !urlOk || !tokenOk {
		logx.Error("下游返回的url或sessionToken字段格式错误")
		return nil, consts.ErrCall
	}

	// 构造响应结果
	result := &show.DownloadEvaluateResp{
		Url:          url,
		SessionToken: sessionToken,
	}

	// 将结果存入缓存
	if err := s.DownloadCacheMapper.Set(ctx, req.Id, result); err != nil {
		logx.Error("存储缓存失败: %v", err)
		// 缓存失败不影响正常返回结果
	} else {
		logx.Info("成功缓存下载链接, id: %s, 缓存时间: 1小时", req.Id)
	}

	return result, nil
}
