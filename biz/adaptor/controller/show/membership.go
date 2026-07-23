package show

import (
	"context"
	"essay-show/biz/adaptor"
	show "essay-show/biz/application/dto/essay/show"
	"essay-show/provider"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// ListMembershipProducts .
// @router /membership/products [GET]
func ListMembershipProducts(ctx context.Context, c *app.RequestContext) {
	var req show.ListMembershipProductsReq
	if err := c.BindAndValidate(&req); err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	p := provider.Get()
	resp, err := p.MembershipService.ListProducts(ctx, &req)
	adaptor.PostProcess(ctx, c, &req, resp, err)
}

// SignMembership .
// @router /membership/sign [POST]
func SignMembership(ctx context.Context, c *app.RequestContext) {
	var req show.SignMembershipReq
	if err := c.BindAndValidate(&req); err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	p := provider.Get()
	resp, err := p.MembershipService.SignMembership(ctx, &req)
	adaptor.PostProcess(ctx, c, &req, resp, err)
}

// GetMembershipStatus .
// @router /membership/status [GET]
func GetMembershipStatus(ctx context.Context, c *app.RequestContext) {
	var req show.GetMembershipStatusReq
	if err := c.BindAndValidate(&req); err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	p := provider.Get()
	resp, err := p.MembershipService.GetStatus(ctx, &req)
	adaptor.PostProcess(ctx, c, &req, resp, err)
}

type platformVirtualPayNotify struct {
	EventType     string `form:"eventType" json:"eventType" query:"eventType"`
	OutTradeNo    string `form:"outTradeNo" json:"outTradeNo" query:"outTradeNo"`
	TransactionID string `form:"transactionId" json:"transactionId" query:"transactionId"`
}

// MembershipNotify 中台虚拟支付道具发货事件回调
// @router /membership/notify [POST]
func MembershipNotify(ctx context.Context, c *app.RequestContext) {
	var notify platformVirtualPayNotify
	if err := c.BindAndValidate(&notify); err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	req := &show.MembershipNotifyReq{
		EventType:     notify.EventType,
		OrderNo:       notify.OutTradeNo,
		TransactionId: notify.TransactionID,
	}

	p := provider.Get()
	resp, err := p.MembershipService.HandleNotify(ctx, req)
	adaptor.PostProcess(ctx, c, req, resp, err)
}
