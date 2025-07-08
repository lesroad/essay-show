package log

import (
	"context"
	"essay-show/biz/application/dto/basic"
	"essay-show/biz/infrastructure/config"
	"essay-show/biz/infrastructure/consts"
	util "essay-show/biz/infrastructure/util/page"
	"time"

	"github.com/zeromicro/go-zero/core/stores/monc"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	prefixKeyCacheKey = "cache:log"
	CollectionName    = "log"
	ErrCollectionName = "err_log"
)

type IMongoMapper interface {
	Insert(ctx context.Context, l *Log) error
	InsertErr(ctx context.Context, l *Log) error
	FindMany(ctx context.Context, userId string, p *basic.PaginationOptions) (logs []*Log, total int64, err error)
	FindOne(ctx context.Context, id string) (l *Log, err error)
	Update(ctx context.Context, l *Log) error
}

type MongoMapper struct {
	conn    *monc.Model
	errConn *monc.Model
}

func NewMongoMapper(config *config.Config) *MongoMapper {
	conn := monc.MustNewModel(config.Mongo.URL, config.Mongo.DB, CollectionName, config.Cache)
	errConn := monc.MustNewModel(config.Mongo.URL, config.Mongo.DB, ErrCollectionName, config.Cache)
	return &MongoMapper{conn: conn, errConn: errConn}
}

func (m *MongoMapper) Insert(ctx context.Context, l *Log) error {
	if l.ID.IsZero() {
		l.ID = primitive.NewObjectID()
		l.CreateTime = time.Now()
	}
	key := prefixKeyCacheKey + l.ID.Hex()
	_, err := m.conn.InsertOne(ctx, key, l)
	return err
}

func (m *MongoMapper) InsertErr(ctx context.Context, l *Log) error {
	if l.ID.IsZero() {
		l.ID = primitive.NewObjectID()
		l.CreateTime = time.Now()
	}
	_, err := m.errConn.InsertOneNoCache(ctx, l)
	return err
}

func (m *MongoMapper) FindMany(ctx context.Context, userId string, p *basic.PaginationOptions) (logs []*Log, total int64, err error) {
	skip, limit := util.ParsePageOpt(p)
	logs = make([]*Log, 0, limit)
	err = m.conn.Find(ctx, &logs,
		bson.M{
			consts.UserID: userId,
		}, &options.FindOptions{
			Skip:  &skip,
			Limit: &limit,
			Sort:  bson.M{consts.CreateTime: -1},
		})
	if err != nil {
		return nil, 0, err
	}

	total, err = m.conn.CountDocuments(ctx, bson.M{
		consts.UserID: userId,
	})
	if err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}

func (m *MongoMapper) FindOne(ctx context.Context, id string) (l *Log, err error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	filter := bson.M{
		consts.ID: oid,
	}

	//key := prefixKeyCacheKey + id

	l = &Log{}
	//err = m.conn.FindOne(ctx, key, l, filter)
	err = m.conn.FindOneNoCache(ctx, l, filter)
	return l, err
}

func (m *MongoMapper) Update(ctx context.Context, l *Log) error {
	key := prefixKeyCacheKey + l.ID.Hex()
	_, err := m.conn.UpdateByID(ctx, key, l.ID, bson.M{"$set": l})
	return err
}
