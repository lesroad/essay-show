package class

import (
	"context"
	"essay-show/biz/infrastructure/config"
	"essay-show/biz/infrastructure/consts"
	"essay-show/biz/infrastructure/util/log"
	"time"

	"github.com/zeromicro/go-zero/core/stores/monc"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Class struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name        string             `bson:"name" json:"name"`
	Description string             `bson:"description" json:"description"`
	CreatorID   string             `bson:"creator_id" json:"creatorId"`
	MemberCount int64              `bson:"member_count" json:"memberCount"`
	CreateTime  time.Time          `bson:"create_time" json:"createTime"`
	UpdateTime  time.Time          `bson:"update_time" json:"updateTime"`
	DeleteTime  time.Time          `bson:"delete_time,omitempty" json:"deleteTime"`
}

const (
	prefixClassCacheKey = "cache:class"
	ClassCollectionName = "class"
)

type MongoMapper struct {
	conn *monc.Model
}

func NewMongoMapper(config *config.Config) *MongoMapper {
	log.Info("NewClassMongoMapper config: %v, collection: %s", config, ClassCollectionName)
	conn := monc.MustNewModel(config.Mongo.URL, config.Mongo.DB, ClassCollectionName, config.Cache)
	return &MongoMapper{
		conn: conn,
	}
}

func (m *MongoMapper) Insert(ctx context.Context, class *Class) error {
	if class.ID.IsZero() {
		class.ID = primitive.NewObjectID()
		class.CreateTime = time.Now()
		class.UpdateTime = class.CreateTime
	}
	_, err := m.conn.InsertOneNoCache(ctx, class)
	return err
}

func (m *MongoMapper) FindOne(ctx context.Context, id string) (*Class, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, consts.ErrInvalidObjectId
	}
	var c Class
	err = m.conn.FindOneNoCache(ctx, &c, bson.M{
		consts.ID: oid,
	})
	if err != nil {
		return nil, consts.ErrNotFound
	}
	return &c, nil
}

func (m *MongoMapper) FindByCreator(ctx context.Context, creatorID string, page, pageSize int64) ([]*Class, int64, error) {
	var classes []*Class
	filter := bson.M{"creator_id": creatorID}

	// 获取总数
	total, err := m.conn.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	// 分页查询
	skip := (page - 1) * pageSize
	err = m.conn.Find(ctx, &classes, filter, &options.FindOptions{
		Skip:  &skip,
		Limit: &pageSize,
		Sort:  bson.M{"create_time": -1},
	})
	if err != nil {
		return nil, 0, err
	}

	return classes, total, nil
}

func (m *MongoMapper) UpdateMemberCount(ctx context.Context, id string, increment int64) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return consts.ErrInvalidObjectId
	}
	_, err = m.conn.UpdateByIDNoCache(ctx, oid, bson.M{
		"$inc": bson.M{
			"member_count": increment,
		},
		"$set": bson.M{
			"update_time": time.Now(),
		},
	})
	return err
}
