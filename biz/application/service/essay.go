package service

import (
	"context"
	"encoding/json"
	"essay-show/biz/adaptor"
	"essay-show/biz/application/dto/essay/apigateway"
	"essay-show/biz/application/dto/essay/show"
	"essay-show/biz/application/dto/essay/stateless"
	"essay-show/biz/infrastructure/cache"
	"essay-show/biz/infrastructure/consts"
	"essay-show/biz/infrastructure/lock"
	"essay-show/biz/infrastructure/repository/log"
	"essay-show/biz/infrastructure/repository/user"
	"essay-show/biz/infrastructure/util"
	logx "essay-show/biz/infrastructure/util/log"
	"fmt"
	"strings"
	"time"

	"github.com/google/wire"
	"github.com/jinzhu/copier"
	"github.com/mitchellh/mapstructure"
)

type IEssayService interface {
	EssayEvaluateStream(ctx context.Context, req *show.EssayEvaluateReq, resultChan chan<- string) error
	APIEssayEvaluateStreamV1(ctx context.Context, req *show.EssayEvaluateReq, resultChan chan<- string) error
	GetEvaluateLogs(ctx context.Context, req *show.GetEssayEvaluateLogsReq) (resp *show.GetEssayEvaluateLogsResp, err error)
	LikeEvaluate(ctx context.Context, req *show.LikeEvaluateReq) (resp *show.Response, err error)
	DownloadEvaluate(ctx context.Context, req *show.DownloadEvaluateReq) (resp *show.DownloadEvaluateResp, err error)
	EvaluateModify(ctx context.Context, req *show.EvaluateModifyReq) (resp *show.Response, err error)
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

// EssayEvaluateStream 流式批改作文
func (s *EssayService) EssayEvaluateStream(ctx context.Context, req *show.EssayEvaluateReq, resultChan chan<- string) error {
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
		client.EvaluateStream(ctx, req.Title, req.Text, req.Grade, &req.TotalScore, req.EssayType, nil, downstreamChan)
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
	meta := adaptor.ExtractUserMeta(ctx)
	if meta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}

	if cachedResp, err := s.DownloadCacheMapper.Get(ctx, req.Id); err == nil {
		logx.Info("缓存命中，直接返回下载链接, id: %s", req.Id)
		return cachedResp, nil
	}

	l, err := s.LogMapper.FindOne(ctx, req.Id)
	if err != nil {
		logx.Error("查询批改记录失败: %v", err)
		return nil, consts.ErrNotFound
	}

	if l.UserId != meta.GetUserId() {
		return nil, consts.ErrNotFound
	}

	user, err := s.UserMapper.FindOne(ctx, meta.GetUserId())
	if err != nil {
		logx.Error("获取用户信息失败: %v", err)
		return nil, consts.ErrNotFound
	}

	var evaluateResult stateless.Evaluate
	if err := json.Unmarshal([]byte(l.Response), &evaluateResult); err != nil {
		logx.Error("解析批改结果失败: %v", err)
		return nil, consts.ErrCall
	}

	downloadData := map[string]any{
		"essay_list": []map[string]any{
			{
				"data": evaluateResult,
			},
		},
		"user_id":   user.Username,
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

// APIEssayEvaluateStreamV1 API网关专用流式批改作文接口
func (s *EssayService) APIEssayEvaluateStreamV1(ctx context.Context, req *show.EssayEvaluateReq, resultChan chan<- string) error {
	downstreamChan := make(chan string, 100)
	var finalResult string
	go func() {
		defer close(downstreamChan)
		client := util.GetHttpClient()
		client.EvaluateStream(ctx, req.Title, req.Text, req.Grade, nil, req.EssayType, nil, downstreamChan)
	}()

	for jsonMessage := range downstreamChan {
		// 对每条流式消息进行校验和过滤
		validatedMessage, jump, err := s.validateAndFilterStreamMessage(jsonMessage)
		if err != nil {
			logx.Error("流式消息校验失败: %v, 原始消息: %s", err, jsonMessage)
			continue
		}
		if jump {
			continue
		}

		var data map[string]any
		if parseErr := json.Unmarshal([]byte(validatedMessage), &data); parseErr != nil {
			logx.Error("解析校验后的JSON消息失败: %v, validatedMessage:%s", parseErr, validatedMessage)
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
	if len(finalResult) == 0 {
		util.SendStreamMessage(resultChan, util.STError, "批改失败", nil)
		return consts.ErrCall
	}

	finalData := map[string]interface{}{
		"code":     0,
		"msg":      "批改完成",
		"response": finalResult,
	}

	util.SendStreamMessage(resultChan, util.STComplete, "批改已完成", finalData)
	return nil
}

// validateAndFilterStreamMessage 校验并过滤流式消息，确保每条消息都符合API网关的数据结构
func (s *EssayService) validateAndFilterStreamMessage(messageJSON string) (string, bool, error) {
	var rawMessage map[string]any
	if err := json.Unmarshal([]byte(messageJSON), &rawMessage); err != nil {
		return "", false, fmt.Errorf("无法解析流式消息JSON: %w", err)
	}
	if rawMessage["type"].(string) == "error" {
		return messageJSON, false, nil
	}

	var result map[string]any
	switch rawMessage["step"].(string) {
	case "essay_info":
		var ei apigateway.EssayContent
		if err := mapstructure.Decode(rawMessage["data"].(map[string]any), &ei); err != nil {
			return "", false, fmt.Errorf("解析批改结果失败: %w", err)
		}
		mapstructure.Decode(ei, &result)
	case "finish":
		var ei apigateway.AllContent
		if err := mapstructure.Decode(rawMessage["data"].(map[string]any), &ei); err != nil {
			return "", false, fmt.Errorf("解析批改结果失败: %w", err)
		}
		mapstructure.Decode(ei, &result)
	case "start":
		result = nil
	case "word_sentence", "grammar", "suggestion", "score", "paragraph", "polishing":
		var ei apigateway.AIEvaluation
		if err := mapstructure.Decode(rawMessage["data"].(map[string]any), &ei); err != nil {
			return "", false, fmt.Errorf("解析批改结果失败: %w", err)
		}
		mapstructure.Decode(ei, &result)
	default: // 过滤不支持的批改项
		return "", true, nil
	}

	validatedMessage := apigateway.StreamMessage{
		Type:    rawMessage["type"].(string),
		Message: rawMessage["message"].(string),
		Data:    result,
	}

	validatedBytes, _ := json.Marshal(validatedMessage)
	return string(validatedBytes), false, nil
}

// EvaluateModify 修改作文评价
func (s *EssayService) EvaluateModify(ctx context.Context, req *show.EvaluateModifyReq) (resp *show.Response, err error) {
	meta := adaptor.ExtractUserMeta(ctx)
	if meta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}

	l, err := s.LogMapper.FindOne(ctx, req.Id)
	if err != nil {
		logx.Error("查询批改记录失败: %v", err)
		return nil, consts.ErrNotFound
	}

	if l.UserId != meta.GetUserId() {
		return nil, consts.ErrNotFound
	}

	var evaluateResult stateless.Evaluate
	if err := json.Unmarshal([]byte(l.Response), &evaluateResult); err != nil {
		logx.Error("解析批改结果失败: %v", err)
		return nil, consts.ErrCall
	}

	getDenominator := func(originalWithTotal string) string {
		parts := strings.Split(originalWithTotal, "/")
		if len(parts) == 2 {
			return parts[1]
		}
		return "100" // 默认分母
	}

	if req.Content != nil {
		if req.Content.Text != nil {
			evaluateResult.AIEvaluation.ScoreEvaluation.Comments.Content = *req.Content.Text
		}
		if req.Content.Score != nil {
			originalDenominator := getDenominator(evaluateResult.AIEvaluation.ScoreEvaluation.Scores.ContentWithTotal)
			evaluateResult.AIEvaluation.ScoreEvaluation.Scores.ContentWithTotal = fmt.Sprintf("%d/%s", *req.Content.Score, originalDenominator)
		}
	}

	if req.Expression != nil {
		if req.Expression.Text != nil {
			evaluateResult.AIEvaluation.ScoreEvaluation.Comments.Expression = *req.Expression.Text
		}
		if req.Expression.Score != nil {
			originalDenominator := getDenominator(evaluateResult.AIEvaluation.ScoreEvaluation.Scores.ExpressionWithTotal)
			evaluateResult.AIEvaluation.ScoreEvaluation.Scores.ExpressionWithTotal = fmt.Sprintf("%d/%s", *req.Expression.Score, originalDenominator)
		}
	}

	if req.Structure != nil {
		if req.Structure.Text != nil {
			evaluateResult.AIEvaluation.ScoreEvaluation.Comments.Structure = *req.Structure.Text
		}
		if req.Structure.Score != nil {
			originalDenominator := getDenominator(evaluateResult.AIEvaluation.ScoreEvaluation.Scores.StructureWithTotal)
			evaluateResult.AIEvaluation.ScoreEvaluation.Scores.StructureWithTotal = fmt.Sprintf("%d/%s", *req.Structure.Score, originalDenominator)
		}
	}

	if req.Development != nil {
		if req.Development.Text != nil {
			evaluateResult.AIEvaluation.ScoreEvaluation.Comments.Development = *req.Development.Text
		}
		if req.Development.Score != nil {
			originalDenominator := getDenominator(evaluateResult.AIEvaluation.ScoreEvaluation.Scores.DevelopmentWithTotal)
			evaluateResult.AIEvaluation.ScoreEvaluation.Scores.DevelopmentWithTotal = fmt.Sprintf("%d/%s", *req.Development.Score, originalDenominator)
		}
	}

	if req.OverallComment != nil {
		if req.OverallComment.Text != nil {
			evaluateResult.AIEvaluation.ScoreEvaluation.Comment = *req.OverallComment.Text
		}
		if req.OverallComment.Score != nil {
			originalDenominator := getDenominator(evaluateResult.AIEvaluation.ScoreEvaluation.Scores.AllWithTotal)
			evaluateResult.AIEvaluation.ScoreEvaluation.Scores.AllWithTotal = fmt.Sprintf("%d/%s", *req.OverallComment.Score, originalDenominator)
		}
	}

	if req.Suggestion != nil {
		evaluateResult.AIEvaluation.SuggestionEvaluation.SuggestionDescription = *req.Suggestion
	}

	l.Status = 1

	modifiedResponse, err := json.Marshal(evaluateResult)
	if err != nil {
		logx.Error("序列化修改后的批改结果失败: %v", err)
		return nil, consts.ErrCall
	}

	l.Response = string(modifiedResponse)
	if err := s.LogMapper.Update(ctx, l); err != nil {
		logx.Error("更新批改记录失败: %v", err)
		return nil, consts.ErrCall
	}

	logx.Info("批改记录修改成功，ID: %s", req.Id)
	return &show.Response{
		Code: 0,
		Msg:  "修改成功",
	}, nil
}
