package show

import (
	"context"
	"essay-show/biz/adaptor"
	show "essay-show/biz/application/dto/essay/show"
	"essay-show/provider"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// ListMbaQuestions .
// @router /mba/questions/list [POST]
func ListMbaQuestions(ctx context.Context, c *app.RequestContext) {
	var req show.ListMbaQuestionsReq
	if err := c.BindAndValidate(&req); err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	p := provider.Get()
	resp, err := p.MbaService.ListMbaQuestions(ctx, &req)
	adaptor.PostProcess(ctx, c, &req, resp, err)
}

// GetMbaQuestion .
// @router /mba/question/get [GET]
func GetMbaQuestion(ctx context.Context, c *app.RequestContext) {
	var req show.GetMbaQuestionReq
	if err := c.BindAndValidate(&req); err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	p := provider.Get()
	resp, err := p.MbaService.GetMbaQuestion(ctx, &req)
	adaptor.PostProcess(ctx, c, &req, resp, err)
}

// SubmitMbaAnswer .
// @router /mba/answer/submit [POST]
func SubmitMbaAnswer(ctx context.Context, c *app.RequestContext) {
	var req show.SubmitMbaAnswerReq
	if err := c.BindAndValidate(&req); err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	p := provider.Get()
	resp, err := p.MbaService.SubmitMbaAnswer(ctx, &req)
	adaptor.PostProcess(ctx, c, &req, resp, err)
}

// GetMbaEvaluate .
// @router /mba/evaluate/get [GET]
func GetMbaEvaluate(ctx context.Context, c *app.RequestContext) {
	var req show.GetMbaEvaluateReq
	if err := c.BindAndValidate(&req); err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	p := provider.Get()
	resp, err := p.MbaService.GetMbaEvaluate(ctx, &req)
	adaptor.PostProcess(ctx, c, &req, resp, err)
}

// ListMbaEvaluates .
// @router /mba/evaluates/list [POST]
func ListMbaEvaluates(ctx context.Context, c *app.RequestContext) {
	var req show.ListMbaEvaluatesReq
	if err := c.BindAndValidate(&req); err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	p := provider.Get()
	resp, err := p.MbaService.ListMbaEvaluates(ctx, &req)
	adaptor.PostProcess(ctx, c, &req, resp, err)
}
