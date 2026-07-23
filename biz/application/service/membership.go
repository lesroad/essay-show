package service

import (
	"context"
	"essay-show/biz/adaptor"
	"essay-show/biz/application/dto/essay/show"
	"essay-show/biz/infrastructure/consts"
	membershipRepo "essay-show/biz/infrastructure/repository/membership"
	userRepo "essay-show/biz/infrastructure/repository/user"
	"essay-show/biz/infrastructure/util"
	log "essay-show/biz/infrastructure/util/log"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/google/wire"
)

type IMembershipService interface {
	ListProducts(ctx context.Context, req *show.ListMembershipProductsReq) (*show.ListMembershipProductsResp, error)
	SignMembership(ctx context.Context, req *show.SignMembershipReq) (*show.SignMembershipResp, error)
	HandleNotify(ctx context.Context, req *show.MembershipNotifyReq) (*show.Response, error)
	GetStatus(ctx context.Context, req *show.GetMembershipStatusReq) (*show.GetMembershipStatusResp, error)
	StartExpiryReminder(ctx context.Context)
}

type MembershipService struct {
	ProductMapper *membershipRepo.ProductMongoMapper
	OrderMapper   *membershipRepo.OrderMongoMapper
	UserMapper    *userRepo.MongoMapper
}

var MembershipServiceSet = wire.NewSet(
	wire.Struct(new(MembershipService), "*"),
	wire.Bind(new(IMembershipService), new(*MembershipService)),
)

func (s *MembershipService) ListProducts(ctx context.Context, req *show.ListMembershipProductsReq) (*show.ListMembershipProductsResp, error) {
	products, err := s.ProductMapper.FindActive(ctx)
	if err != nil {
		log.Error("ListProducts error: %v", err)
		return &show.ListMembershipProductsResp{Code: -1, Msg: "查询失败"}, nil
	}
	var pbProducts []*show.MembershipProduct
	for _, p := range products {
		pbProducts = append(pbProducts, &show.MembershipProduct{
			Id:           p.ID.Hex(),
			Name:         p.Name,
			DurationDays: int32(p.DurationDays),
			Price:        fmt.Sprintf("%d", p.PriceFen),
		})
	}
	return &show.ListMembershipProductsResp{Code: 0, Msg: "success", Products: pbProducts}, nil
}

// SignMembership 发起一次会员购买：生成本地订单，再向中台请求小程序虚拟支付所需的签名参数，
// 交由前端调用 wx.requestVirtualPayment 完成支付。允许在会员有效期内提前购买（叠加时长）。
func (s *MembershipService) SignMembership(ctx context.Context, req *show.SignMembershipReq) (*show.SignMembershipResp, error) {
	meta := adaptor.ExtractUserMeta(ctx)
	if meta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}

	product, err := s.ProductMapper.FindOne(ctx, req.ProductId)
	if err != nil || product.Status != 1 {
		return nil, consts.ErrProductNotFound
	}

	orderNo := uuid.NewString()
	order := &membershipRepo.MembershipOrder{
		OrderNo:      orderNo,
		UserID:       meta.GetUserId(),
		ProductID:    req.ProductId,
		AmountFen:    product.PriceFen,
		DurationDays: product.DurationDays,
		Status:       consts.MembershipOrderStatusPending,
	}
	if err := s.OrderMapper.Insert(ctx, order); err != nil {
		log.Error("SignMembership Insert order error: %v", err)
		return nil, consts.ErrCall
	}

	signData, paySig, signature, err := util.GetHttpClient().VirtualPaySign(ctx, meta.GetUserId(), req.JsCode, req.ProductId, product.PriceFen, orderNo)
	if err != nil {
		log.Error("SignMembership VirtualPaySign error: %v", err)
		return nil, consts.ErrPurchaseMembershipFailed
	}

	return &show.SignMembershipResp{
		Code:      0,
		Msg:       "支付参数获取成功，请在小程序完成支付",
		OrderNo:   orderNo,
		SignData:  signData,
		PaySig:    paySig,
		Signature: signature,
	}, nil
}

func (s *MembershipService) HandleNotify(ctx context.Context, req *show.MembershipNotifyReq) (*show.Response, error) {
	switch req.EventType {
	case "deliver_success":
		return s.handleDeliverSuccess(ctx, req)
	default:
		log.Error("HandleNotify unknown event_type: %s", req.EventType)
		return &show.Response{Code: 0, Msg: "ok"}, nil
	}
}

func (s *MembershipService) handleDeliverSuccess(ctx context.Context, req *show.MembershipNotifyReq) (*show.Response, error) {
	order, err := s.OrderMapper.FindByOrderNo(ctx, req.OrderNo)
	if err != nil {
		log.Error("handleDeliverSuccess FindByOrderNo error: %v, orderNo: %s", err, req.OrderNo)
		return &show.Response{Code: -1, Msg: "order not found"}, nil
	}
	if order.Status == consts.MembershipOrderStatusSuccess {
		return &show.Response{Code: 0, Msg: "ok"}, nil
	}

	u, err := s.UserMapper.FindOne(ctx, order.UserID)
	if err != nil {
		log.Error("handleDeliverSuccess FindUser error: %v", err)
		return &show.Response{Code: -1, Msg: "user not found"}, nil
	}

	base := u.VipExpireTime
	if base.Before(time.Now()) {
		base = time.Now()
	}
	periodEnd := base.AddDate(0, 0, order.DurationDays)

	if err := s.UserMapper.UpdateVip(ctx, order.UserID, periodEnd); err != nil {
		log.Error("handleDeliverSuccess UpdateVip error: %v", err)
		return &show.Response{Code: -1, Msg: "activate vip failed"}, nil
	}
	if err := s.OrderMapper.UpdateStatus(ctx, req.OrderNo, consts.MembershipOrderStatusSuccess, req.TransactionId, base, periodEnd); err != nil {
		log.Error("handleDeliverSuccess UpdateStatus error: %v", err)
	}
	log.Info("handleDeliverSuccess: VIP activated/extended for user %s, expire %s", order.UserID, periodEnd.Format(time.RFC3339))
	return &show.Response{Code: 0, Msg: "ok"}, nil
}

func (s *MembershipService) GetStatus(ctx context.Context, req *show.GetMembershipStatusReq) (*show.GetMembershipStatusResp, error) {
	meta := adaptor.ExtractUserMeta(ctx)
	if meta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}

	u, err := s.UserMapper.FindOne(ctx, meta.GetUserId())
	if err != nil {
		return nil, consts.ErrNotFound
	}

	isVip := userRepo.IsVipActive(u)
	var expireTs int64
	if isVip {
		expireTs = u.VipExpireTime.Unix()
	}
	return &show.GetMembershipStatusResp{
		Code:          0,
		Msg:           "success",
		IsVip:         isVip,
		VipExpireTime: expireTs,
	}, nil
}

// StartExpiryReminder 启动到期提醒定时器，仅在临期时通过微信订阅消息提醒用户手动续购
func (s *MembershipService) StartExpiryReminder(ctx context.Context) {
	log.Info("启动会员到期提醒定时器")
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.remindExpiringUsers(context.Background())
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (s *MembershipService) remindExpiringUsers(ctx context.Context) {
	users, err := s.UserMapper.FindUsersNearExpiry(ctx, time.Now(), time.Now().Add(24*time.Hour))
	if err != nil {
		log.Error("remindExpiringUsers FindUsersNearExpiry error: %v", err)
		return
	}
	for _, u := range users {
		// TODO: 通过微信小程序订阅消息提醒用户会员即将到期，需前端配合申请订阅消息模板 ID。
		log.Info("会员即将到期提醒: userId=%s, expire=%s", u.ID.Hex(), u.VipExpireTime.Format(time.RFC3339))
	}
}
