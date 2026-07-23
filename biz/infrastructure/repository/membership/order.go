package membership

import (
	"context"
	"essay-show/biz/infrastructure/config"
	"essay-show/biz/infrastructure/consts"
	"time"

	"github.com/zeromicro/go-zero/core/stores/monc"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const orderCollection = "membership_order"

// MembershipOrder essay-show 业务订单（一次性购买会员时长，与中台虚拟支付订单通过 order_no 关联）
type MembershipOrder struct {
	ID              primitive.ObjectID `bson:"_id,omitempty"`
	OrderNo         string             `bson:"order_no"`
	UserID          string             `bson:"user_id"`
	ProductID       string             `bson:"product_id"`
	AmountFen       int64              `bson:"amount_fen"`
	DurationDays    int                `bson:"duration_days"`
	Status          int                `bson:"status"` // 0=待处理 1=成功 2=失败
	WxTransactionID string             `bson:"wx_transaction_id,omitempty"`
	PeriodStart     time.Time          `bson:"period_start,omitempty"`
	PeriodEnd       time.Time          `bson:"period_end,omitempty"`
	CreateTime      time.Time          `bson:"create_time"`
	UpdateTime      time.Time          `bson:"update_time"`
}

type OrderMongoMapper struct {
	conn *monc.Model
}

func NewOrderMongoMapper(cfg *config.Config) *OrderMongoMapper {
	conn := monc.MustNewModel(cfg.Mongo.URL, cfg.Mongo.DB, orderCollection, cfg.Cache)
	return &OrderMongoMapper{conn: conn}
}

func (m *OrderMongoMapper) Insert(ctx context.Context, o *MembershipOrder) error {
	if o.ID.IsZero() {
		o.ID = primitive.NewObjectID()
		o.CreateTime = time.Now()
		o.UpdateTime = o.CreateTime
	}
	_, err := m.conn.InsertOneNoCache(ctx, o)
	return err
}

func (m *OrderMongoMapper) FindByOrderNo(ctx context.Context, orderNo string) (*MembershipOrder, error) {
	var o MembershipOrder
	err := m.conn.FindOneNoCache(ctx, &o, bson.M{"order_no": orderNo})
	if err != nil {
		return nil, consts.ErrNotFound
	}
	return &o, nil
}

func (m *OrderMongoMapper) UpdateStatus(ctx context.Context, orderNo string, status int, transactionID string, periodStart, periodEnd time.Time) error {
	update := bson.M{
		"status":      status,
		"update_time": time.Now(),
	}
	if transactionID != "" {
		update["wx_transaction_id"] = transactionID
	}
	if !periodStart.IsZero() {
		update["period_start"] = periodStart
		update["period_end"] = periodEnd
	}
	_, err := m.conn.UpdateOneNoCache(ctx, bson.M{"order_no": orderNo}, bson.M{"$set": update})
	return err
}
