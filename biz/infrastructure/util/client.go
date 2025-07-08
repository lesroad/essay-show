package util

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"essay-show/biz/infrastructure/config"
	"essay-show/biz/infrastructure/consts"
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
			// 为流式传输优化的配置
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
		// 其他SSE字段（event:, id:, retry:）暂时忽略
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

// SignUp 用于用户初始化
func (c *HttpClient) SignUp(ctx context.Context, authType string, authId string, verifyCode *string) (map[string]interface{}, error) {

	body := make(map[string]interface{})
	body["authType"] = authType
	body["authId"] = authId
	body["verifyCode"] = *verifyCode
	body["appId"] = consts.AppId

	header := make(map[string]string)
	header["Content-Type"] = consts.ContentTypeJson
	header["Charset"] = consts.CharSetUTF8

	// 如果是测试环境则向测试环境的中台发送请求
	if config.GetConfig().State == "test" {
		header["X-Xh-Env"] = "test"
	}

	resp, err := c.SendRequest(ctx, consts.Post, config.GetConfig().Api.PlatfromURL+"/sts/sign_in", header, body)
	if err != nil {
		return nil, err
	}
	return resp, nil
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

	// 如果是测试环境则向测试环境中台发送请求
	if config.GetConfig().State == "test" {
		header["X-Xh-Env"] = "test"
	}

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

// SetPassword 用于用户登录
func (c *HttpClient) SetPassword(ctx context.Context, userId string, password string) (map[string]interface{}, error) {

	body := make(map[string]interface{})
	body["password"] = password
	body["appId"] = consts.AppId
	body["userId"] = userId

	header := make(map[string]string)
	header["Content-Type"] = consts.ContentTypeJson
	header["Charset"] = consts.CharSetUTF8

	// 如果是测试环境则向测试环境中台发送请求
	if config.GetConfig().State == "test" {
		header["X-Xh-Env"] = "test"
	}

	resp, err := c.SendRequest(ctx, consts.Post, config.GetConfig().Api.PlatfromURL+"/sts/set_password", header, body)
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

// Evaluate 批改作文，支持context和链路追踪
func (c *HttpClient) Evaluate(ctx context.Context, title string, text string, grade *int64, essayType *string) (map[string]interface{}, error) {
	data := make(map[string]interface{})
	data["title"] = title
	data["content"] = text
	if grade != nil {
		data["grade"] = *grade
	}
	if essayType != nil {
		data["essayType"] = *essayType
	}

	header := make(map[string]string)
	header["Content-Type"] = consts.ContentTypeJson
	header["Charset"] = consts.CharSetUTF8

	resp, err := c.SendRequest(ctx, consts.Post, config.GetConfig().Api.EvaluateUrl, header, data)
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

	resp, err := c.SendRequest(ctx, consts.Post, config.GetConfig().Api.TitleUrlOcr, header, body)
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

// EvaluateStream 流式批改作文，支持context和链路追踪
func (c *HttpClient) EvaluateStream(ctx context.Context, title string, text string, grade *int64, essayType *string, resultChan chan<- string) error {
	// 准备请求参数
	data := make(map[string]interface{})
	data["title"] = title
	data["content"] = text
	if grade != nil {
		data["grade"] = *grade
	}
	if essayType != nil {
		data["essayType"] = *essayType
	}

	// 准备请求头
	headers := make(map[string]string)
	headers["Content-Type"] = "application/json"

	// 构建完整的URL
	url := config.GetConfig().Api.EvaluateUrl + "/stream"

	return c.SendRequestStream(ctx, "POST", url, headers, data, resultChan)
}

// EssayPolish 批改结果下载，支持context和链路追踪
func (c *HttpClient) EssayPolish(ctx context.Context, data map[string]any) (map[string]any, error) {
	header := make(map[string]string)
	header["Content-Type"] = "application/json"
	header["Charset"] = "utf-8"
	resp, err := c.SendRequest(ctx, consts.Post, config.GetConfig().Api.DownloadURL, header, data)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
