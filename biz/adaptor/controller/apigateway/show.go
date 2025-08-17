package apigateway

import (
	"context"
	"encoding/json"
	"essay-show/biz/application/dto/essay/show"
	"essay-show/biz/infrastructure/util"
	"essay-show/biz/infrastructure/util/log"
	"essay-show/provider"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/cloudwego/hertz/pkg/protocol/sse"
)

// APIEssayEvaluateStreamV1 - API网关专用的作文批改流式接口 (v1.0)
func APIEssayEvaluateStreamV1(ctx context.Context, c *app.RequestContext) {
	var req show.EssayEvaluateReq
	if err := c.BindAndValidate(&req); err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	log.CtxInfo(ctx, "[API-Gateway-V1] req=%s", util.JSONF(&req))

	c.SetStatusCode(http.StatusOK)
	w := sse.NewWriter(c)

	resultChan := make(chan string, 100)

	go func(ctx context.Context) {
		p := provider.Get()
		defer close(resultChan)
		p.EssayService.APIEssayEvaluateStreamV1(ctx, &req, resultChan)
	}(ctx)

	for jsonMessage := range resultChan {
		err := w.WriteEvent("", "", []byte(jsonMessage))
		if err != nil {
			log.Error("发送SSE事件失败: %v", err)
			break
		}

		var msgData util.StreamMessage
		json.Unmarshal([]byte(jsonMessage), &msgData)
		if msgData.Type == util.STComplete {
			log.CtxInfo(ctx, "[API-Gateway-V1] 批改完成")
			break
		}
		if msgData.Type == util.STError {
			log.CtxInfo(ctx, "[API-Gateway-V1] 批改错误: %+v", msgData)
			break
		}
	}
}

// APIOCRV1 - API网关专用的OCR接口 (v1.0)
// 简化版本：无需认证、无需校验次数
// 专门用于API网关调用，只负责核心的OCR识别功能
func APIOCRV1(ctx context.Context, c *app.RequestContext) {
	var req show.OCRReq
	if err := c.BindAndValidate(&req); err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	log.CtxInfo(ctx, "[API-Gateway-OCR-V1] req=%s", util.JSONF(&req))

	p := provider.Get()
	resp, err := p.StsService.APIOCRV1(ctx, &req)
	if err != nil {
		log.Error("[API-Gateway-OCR-V1] OCR失败: %v", err)
		c.JSON(consts.StatusInternalServerError, map[string]interface{}{
			"code":    50000,
			"message": "OCR识别失败",
			"error":   err.Error(),
		})
		return
	}

	log.CtxInfo(ctx, "[API-Gateway-OCR-V1] OCR成功")
	c.JSON(consts.StatusOK, resp)
}
