package membership

import (
	"context"
	"essay-show/biz/infrastructure/config"
	"essay-show/biz/infrastructure/consts"
	"time"

	"github.com/zeromicro/go-zero/core/stores/monc"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const productCollection = "membership_product"

type MembershipProduct struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"`
	Name         string             `bson:"name"`
	DurationDays int                `bson:"duration_days"`
	PriceFen     int64              `bson:"price_fen"`
	Status       int                `bson:"status"` // 1=上架 0=下架
	CreateTime   time.Time          `bson:"create_time"`
	UpdateTime   time.Time          `bson:"update_time"`
}

type ProductMongoMapper struct {
	conn *monc.Model
}

func NewProductMongoMapper(cfg *config.Config) *ProductMongoMapper {
	conn := monc.MustNewModel(cfg.Mongo.URL, cfg.Mongo.DB, productCollection, cfg.Cache)
	return &ProductMongoMapper{conn: conn}
}

func (m *ProductMongoMapper) FindActive(ctx context.Context) ([]*MembershipProduct, error) {
	var products []*MembershipProduct
	err := m.conn.Find(ctx, &products, bson.M{"status": 1}, &options.FindOptions{
		Sort: bson.M{"price_fen": 1},
	})
	if err != nil {
		return nil, err
	}
	return products, nil
}

func (m *ProductMongoMapper) FindOne(ctx context.Context, id string) (*MembershipProduct, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, consts.ErrInvalidObjectId
	}
	var p MembershipProduct
	err = m.conn.FindOneNoCache(ctx, &p, bson.M{consts.ID: oid})
	if err != nil {
		return nil, consts.ErrNotFound
	}
	return &p, nil
}
