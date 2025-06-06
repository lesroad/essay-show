package feedback

import (
	"context"
	"essay-show/biz/infrastructure/config"
	"time"

	"github.com/zeromicro/go-zero/core/stores/monc"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	prefixKeyCacheKey = "cache:feedback"
	CollectionName    = "feedback"
)

type IMongoMapper interface {
	Insert(ctx context.Context, f *Feedback) error
}

type MongoMapper struct {
	conn *monc.Model
}

func NewMongoMapper(config *config.Config) *MongoMapper {
	conn := monc.MustNewModel(config.Mongo.URL, config.Mongo.DB, CollectionName, config.Cache)
	return &MongoMapper{conn: conn}
}

func (m *MongoMapper) Insert(ctx context.Context, f *Feedback) error {
	if f.ID.IsZero() {
		f.ID = primitive.NewObjectID()
		f.CreateTime = time.Now()
		f.UpdateTime = time.Now()
	}
	_, err := m.conn.InsertOneNoCache(ctx, f)
	return err
}
