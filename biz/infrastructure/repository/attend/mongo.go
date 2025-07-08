package attend

import (
	"context"
	"errors"
	"essay-show/biz/infrastructure/config"
	"essay-show/biz/infrastructure/consts"
	"time"

	"github.com/zeromicro/go-zero/core/stores/monc"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	prefixKeyCacheKey = "cache:attend"
	CollectionName    = "attend"
)

type IMongoMapper interface {
	Insert(ctx context.Context, a *Attend) error
	InsertZeroOne(ctx context.Context, userId string) (*Attend, error)
	FindLatestOneByUserId(ctx context.Context, userId string) (a *Attend, err error)
	Update(ctx context.Context, a *Attend) error
	FindByYearAndMonth(ctx context.Context, userId string, year int, month int) (as []*Attend, total int64, err error)
}

type MongoMapper struct {
	conn *monc.Model
}

func NewMongoMapper(config *config.Config) *MongoMapper {
	conn := monc.MustNewModel(config.Mongo.URL, config.Mongo.DB, CollectionName, config.Cache)
	return &MongoMapper{conn: conn}
}

func (m *MongoMapper) InsertZeroOne(ctx context.Context, userId string) (*Attend, error) {
	a := &Attend{
		ID:        primitive.NewObjectID(),
		UserId:    userId,
		Timestamp: time.Time{},
	}
	_, err := m.conn.InsertOneNoCache(ctx, a)
	return a, err
}

func (m *MongoMapper) Insert(ctx context.Context, a *Attend) error {
	_, err := m.conn.InsertOneNoCache(ctx, a)
	return err
}

func (m *MongoMapper) FindLatestOneByUserId(ctx context.Context, userId string) (a *Attend, err error) {
	a = &Attend{}
	// 根据timestamp获取最新的签到记录
	err = m.conn.FindOneNoCache(ctx, a, bson.M{consts.UserID: userId},
		options.FindOne().SetSort(bson.M{consts.Timestamp: -1}))
	switch {
	case err == nil:
		return a, nil
	case errors.Is(err, mongo.ErrNoDocuments):
		return nil, consts.ErrNotFound
	default:
		return nil, err
	}
}

func (m *MongoMapper) Update(ctx context.Context, a *Attend) error {
	_, err := m.conn.UpdateByIDNoCache(ctx, a.ID, bson.M{"$set": a})
	return err
}

func (m *MongoMapper) FindByYearAndMonth(ctx context.Context, userId string, year int, month int) (as []*Attend, total int64, err error) {
	as = make([]*Attend, 0)
	// 构造这个月的开始和结束
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.Local)
	end := start.AddDate(0, 1, 0).Add(-time.Nanosecond)
	// 找到这个月所有的签到记录
	err = m.conn.Find(ctx, &as, bson.M{
		consts.UserID: userId,
		consts.Timestamp: bson.M{
			"$gte": start,
			"$lt":  end,
		},
	})
	if err != nil {
		return nil, 0, err
	}

	// 用户签到总数
	total, err = m.conn.CountDocuments(ctx, bson.M{consts.UserID: userId})
	if err != nil {
		return nil, 0, err
	}
	return as, total, nil
}
