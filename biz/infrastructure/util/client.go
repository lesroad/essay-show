package util

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"essay-show/biz/infrastructure/config"
	"essay-show/biz/infrastructure/consts"
	"essay-show/biz/infrastructure/repository/class"
	"essay-show/biz/infrastructure/repository/homework"
	"essay-show/biz/infrastructure/util/log"
	"fmt"
	"io"
	"net/http"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var client *HttpClient

// HttpClient 是一个简单的 HTTP 客户端
type HttpClient struct {
	Client *http.Client
	Config *config.Config
}

// NewHttpClient 创建一个新的 HttpClient 实例，集成OpenTelemetry
func NewHttpClient() *HttpClient {
	return &HttpClient{
		Client: &http.Client{
			Timeout: 0, // 禁用超时，因为流式请求可能持续很长时间
		},
	}
}

func GetHttpClient() *HttpClient {
	if client == nil {
		client = NewHttpClient()
	}
	return client
}

// SendRequest 发送 HTTP 请求
func (c *HttpClient) SendRequest(ctx context.Context, method, url string, headers map[string]string, body interface{}) (map[string]interface{}, error) {
	// 创建子span用于追踪HTTP请求
	// tracer := otel.Tracer("essay-show-http-client")
	// ctx, span := tracer.Start(ctx, fmt.Sprintf("HTTP %s", method))
	// defer span.End()
	span := trace.SpanFromContext(ctx)

	// 将 body 序列化为 JSON
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("请求体序列化失败: %w", err)
	}

	// 创建新的请求
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// 发送请求
	resp, err := c.Client.Do(req)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("发送请求失败: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Error("关闭请求失败", closeErr)
		}
	}()

	// 记录响应状态码
	span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))

	// 读取响应
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 检查响应状态码
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err := fmt.Errorf("unexpected status code: %d, response body: %s", resp.StatusCode, responseBody)
		span.RecordError(err)
		return nil, err
	}

	// 反序列化响应体
	var responseMap map[string]interface{}
	if err := json.Unmarshal(responseBody, &responseMap); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("反序列化响应失败: %w", err)
	}

	return responseMap, nil
}

// SendRequestStream 发送流式 HTTP 请求，支持context和链路追踪
// 使用标准HTTP客户端而非Hertz客户端，确保trace context自动传递
func (c *HttpClient) SendRequestStream(ctx context.Context, method, url string, headers map[string]string, body interface{}, resultChan chan<- string) error {
	// 创建span用于追踪流式HTTP请求
	tracer := otel.Tracer("essay-show-http-client")
	ctx, span := tracer.Start(ctx, "SendRequestStream")
	defer span.End()

	// 添加span属性
	span.SetAttributes(
		attribute.String("http.method", method),
		attribute.String("http.url", url),
		attribute.String("component", "http-client-stream"),
		attribute.Bool("http.stream", true),
	)

	// 序列化请求体
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("请求体序列化失败: %w", err)
	}

	// 创建HTTP请求，使用标准HTTP客户端
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置SSE相关的请求头
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	// 设置自定义请求头
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// 发送请求
	resp, err := c.Client.Do(req)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("发送请求失败: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Error("关闭流式响应失败: %v", closeErr)
		}
	}()

	// 记录响应状态码
	span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))

	// 检查响应状态码
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err := fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		span.RecordError(err)
		return err
	}

	// 检查Content-Type是否为SSE
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/event-stream") {
		log.Info("Warning: Content-Type is not text/event-stream, got: %s", contentType)
	}

	// 使用bufio.Scanner逐行读取SSE流
	scanner := bufio.NewScanner(resp.Body)
	var eventData strings.Builder

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			span.RecordError(ctx.Err())
			return ctx.Err()
		default:
		}

		line := scanner.Text()

		// SSE协议：空行表示事件结束
		if line == "" {
			if eventData.Len() > 0 {
				data := eventData.String()
				eventData.Reset()

				// 发送到结果通道
				select {
				case resultChan <- data:
				case <-ctx.Done():
					span.RecordError(ctx.Err())
					return ctx.Err()
				}

				// 解析事件数据，检查是否应该停止
				var eventMap map[string]interface{}
				if parseErr := json.Unmarshal([]byte(data), &eventMap); parseErr == nil {
					if msgType, ok := eventMap["type"].(string); ok {
						if msgType == "complete" || msgType == "final" || msgType == "end" {
							log.Info("收到流式完成信号: %s", msgType)
							return nil
						}
						if msgType == "error" {
							err := fmt.Errorf("收到流式错误消息: %v", eventMap)
							span.RecordError(err)
							return nil // 不返回错误，让上层处理
						}
					}
				}
			}
			continue
		}

		// 处理SSE事件行
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if eventData.Len() > 0 {
				eventData.WriteString("\n")
			}
			eventData.WriteString(data)
		}
	}

	// 检查scanner是否遇到错误
	if err := scanner.Err(); err != nil {
		span.RecordError(err)
		return fmt.Errorf("读取SSE流失败: %w", err)
	}

	// 处理最后一个事件（如果没有以空行结尾）
	if eventData.Len() > 0 {
		data := eventData.String()
		select {
		case resultChan <- data:
		case <-ctx.Done():
			span.RecordError(ctx.Err())
			return ctx.Err()
		}
	}

	return nil
}

// SignIn 用于用户登录
func (c *HttpClient) SignIn(ctx context.Context, authType string, authId string, verifyCode *string, password *string) (map[string]interface{}, error) {

	body := make(map[string]interface{})
	body["authType"] = authType
	body["authId"] = authId
	if verifyCode != nil {
		body["verifyCode"] = *verifyCode
	}
	if password != nil {
		body["password"] = *password
	}
	body["appId"] = consts.AppId

	header := make(map[string]string)
	header["Content-Type"] = consts.ContentTypeJson
	header["Charset"] = consts.CharSetUTF8

	resp, err := c.SendRequest(ctx, consts.Post, config.GetConfig().Api.PlatfromURL+"/sts/sign_in", header, body)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *HttpClient) BindAuth(ctx context.Context, authType string, authId string, verifyCode *string, userId string) (map[string]interface{}, error) {
	body := make(map[string]interface{})
	body["authType"] = authType
	body["authId"] = authId
	if verifyCode != nil {
		body["verifyCode"] = *verifyCode
	}
	body["appId"] = consts.AppId
	body["userId"] = userId

	header := make(map[string]string)
	header["Content-Type"] = consts.ContentTypeJson
	header["Charset"] = consts.CharSetUTF8

	resp, err := c.SendRequest(ctx, consts.Post, config.GetConfig().Api.PlatfromURL+"/sts/add_auth", header, body)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// SendVerifyCode SetPassword 用于用户登录
func (c *HttpClient) SendVerifyCode(ctx context.Context, authType string, authId string) (map[string]interface{}, error) {

	body := make(map[string]interface{})
	body["authType"] = authType
	body["authId"] = authId

	header := make(map[string]string)
	header["Content-Type"] = consts.ContentTypeJson
	header["Charset"] = consts.CharSetUTF8

	// 如果是测试环境则向测试环境中台发送请求
	if config.GetConfig().State == "test" {
		header["X-Xh-Env"] = "test"
	}

	resp, err := c.SendRequest(ctx, consts.Post, config.GetConfig().Api.PlatfromURL+"/sts/send_verify_code", header, body)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// TitleUrlOCR ocr - 带标题
func (c *HttpClient) TitleUrlOCR(ctx context.Context, images []string, left string) (map[string]interface{}, error) {
	body := make(map[string]interface{})
	// 图片url列表
	body["images"] = images
	// 保留类型
	if len(left) > 0 {
		body["leftType"] = left
	}

	header := make(map[string]string)
	header["Content-Type"] = consts.ContentTypeJson
	if config.GetConfig().State == "test" {
		header["X-Xh-Env"] = "test"
	}

	resp, err := c.SendRequest(ctx, consts.Post, config.GetConfig().Api.StatelessURL+"/sts/ocr/title/ark/url", header, body)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *HttpClient) GetEssayInfo(ctx context.Context, essay string, title string) (map[string]interface{}, error) {
	body := make(map[string]interface{})
	body["essay"] = essay
	body["title"] = title

	header := make(map[string]string)
	header["Content-Type"] = consts.ContentTypeJson

	resp, err := c.SendRequest(ctx, consts.Post, config.GetConfig().Api.AlgorithmURL+"/essay_info", header, body)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *HttpClient) GenCosSts(ctx context.Context, path string) (map[string]any, error) {
	body := make(map[string]any)
	body["path"] = path

	header := make(map[string]string)
	header["Content-Type"] = consts.ContentTypeJson
	if config.GetConfig().State == "test" {
		header["X-Xh-Env"] = "test"
	}

	URL := config.GetConfig().Api.PlatfromURL + "/sts/gen_cos_sts"
	resp, err := c.SendRequest(ctx, consts.Post, URL, header, body)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *HttpClient) SendWechatMessage(ctx context.Context, userId, templateId string, templateData map[string]string, page *string) (map[string]any, error) {
	body := make(map[string]any)
	body["userId"] = userId
	body["templateId"] = templateId
	body["templateData"] = templateData
	if page != nil && *page != "" {
		body["page"] = *page
	}

	if config.GetConfig().State == "test" {
		body["miniProgramState"] = "trial"
	} else {
		body["miniProgramState"] = "formal"
	}

	header := make(map[string]string)
	header["Content-Type"] = consts.ContentTypeJson
	header["Charset"] = consts.CharSetUTF8

	url := config.GetConfig().Api.PlatfromURL + "/sts/send_wechat_message"
	resp, err := c.SendRequest(ctx, consts.Post, url, header, body)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *HttpClient) GenSignedUrl(ctx context.Context, secretId, secretKey string, method string, path string) (map[string]any, error) {
	body := make(map[string]any)
	body["secretId"] = secretId
	body["secretKey"] = secretKey
	body["method"] = method
	body["path"] = path

	header := make(map[string]string)
	header["Content-Type"] = consts.ContentTypeJson
	if config.GetConfig().State == "test" {
		header["X-Xh-Env"] = "test"
	}

	URL := config.GetConfig().Api.PlatfromURL + "/sts/gen_signed_url"
	resp, err := c.SendRequest(ctx, consts.Post, URL, header, body)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *HttpClient) GenerateUrlLink(ctx context.Context, appId string, path *string, query *string) (map[string]any, error) {
	body := make(map[string]any)
	body["appId"] = appId
	if path != nil && *path != "" {
		body["path"] = *path
	}
	if query != nil && *query != "" {
		body["query"] = *query
	}

	if config.GetConfig().State == "test" {
		body["miniProgramState"] = "trial"
	} else {
		body["miniProgramState"] = "formal"
	}

	header := make(map[string]string)
	header["Content-Type"] = consts.ContentTypeJson
	header["Charset"] = consts.CharSetUTF8

	url := config.GetConfig().Api.PlatfromURL + "/sts/generate_url_link"
	resp, err := c.SendRequest(ctx, consts.Post, url, header, body)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// EvaluateStream 流式批改作文，支持context和链路追踪
// ScoreRatio 自定义分项打分比例
type ScoreRatio struct {
	Content     int `json:"content"`     // 内容分数
	Expression  int `json:"expression"`  // 表达分数
	Structure   int `json:"structure"`   // 结构分数（初中）
	Development int `json:"development"` // 发展分数（高中）
}

// CalculateScoreRatio 自动计算分项打分比例（总分除以3）
// grade: 年级(1-12)
// totalScore: 总分
// 返回: 分项打分比例
func CalculateScoreRatio(grade int64, totalScore int64) *ScoreRatio {
	baseScore := int(totalScore / 3)
	remainder := int(totalScore % 3)

	contentScore := baseScore
	expressionScore := baseScore
	thirdScore := baseScore

	// 将余数分配给第一项（内容分）
	if remainder > 0 {
		contentScore += remainder
	}

	ratio := &ScoreRatio{
		Content:    contentScore,
		Expression: expressionScore,
	}

	// 根据年级判断使用结构分（初中）还是发展分（高中）
	// 1-9年级为初中及以下，使用结构分；10-12年级为高中，使用发展分
	if grade <= 9 {
		ratio.Structure = thirdScore
	} else {
		ratio.Development = thirdScore
	}

	return ratio
}

func (c *HttpClient) EvaluateStream(ctx context.Context, title string, text string, grade, totalScore *int64, essayType *string, prompt *string, standard *string, ratio *ScoreRatio, resultChan chan<- string) error {
	data := make(map[string]interface{})
	data["title"] = title
	data["content"] = text
	if grade != nil {
		data["grade"] = *grade
	}
	if essayType != nil {
		data["essayType"] = *essayType
	}
	if prompt != nil {
		data["prompt"] = *prompt
	}
	if totalScore != nil {
		data["totalScore"] = totalScore
	}

	if standard != nil {
		data["standard"] = *standard
	}

	if ratio != nil {
		data["contentScore"] = int64(ratio.Content)
		data["expressionScore"] = int64(ratio.Expression)
		if ratio.Structure > 0 {
			data["structureScore"] = int64(ratio.Structure)
		}
		if ratio.Development > 0 {
			data["developmentScore"] = int64(ratio.Development)
		} else {
			data["developmentScore"] = 0
		}
	}

	headers := make(map[string]string)
	headers["Content-Type"] = "application/json"

	url := config.GetConfig().Api.StatelessURL + "/evaluate/stream"

	return c.SendRequestStream(ctx, "POST", url, headers, data, resultChan)
}

func (c *HttpClient) EssayPolish(ctx context.Context, data map[string]any) (map[string]any, error) {
	header := make(map[string]string)
	header["Content-Type"] = "application/json"
	header["Charset"] = "utf-8"
	resp, err := c.SendRequest(ctx, consts.Post, config.GetConfig().Api.AlgorithmURL+"/essay_polish", header, data)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *HttpClient) LessonPlan(ctx context.Context, classInfo *class.Class, homework *homework.Homework, essayList []map[string]any) (map[string]any, error) {
	lessonPlanData := map[string]any{
		"class_id":        classInfo.Name,
		"grade":           homework.Grade,
		"last_topic":      "",
		"lesson_duration": 40,
		"essays":          essayList,
	}

	header := make(map[string]string)
	header["Content-Type"] = "application/json"
	header["Charset"] = "utf-8"
	resp, err := c.SendRequest(ctx, consts.Post, config.GetConfig().Api.AlgorithmURL+"/lesson_generate", header, lessonPlanData)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *HttpClient) AnalyzeClassStatistics(ctx context.Context, data map[string]any) (map[string]any, error) {
	header := make(map[string]string)
	header["Content-Type"] = "application/json"
	header["Charset"] = "utf-8"

	url := config.GetConfig().Api.StatelessURL + "/statistics/class"
	resp, err := c.SendRequest(ctx, consts.Post, url, header, data)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
