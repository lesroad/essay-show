package user

import (
	"context"
	"errors"
	"essay-show/biz/infrastructure/config"
	"essay-show/biz/infrastructure/consts"
	"essay-show/biz/infrastructure/util/log"
	"time"

	"github.com/zeromicro/go-zero/core/stores/monc"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	prefixUserCacheKey = "cache:user"
	CollectionName     = "user"
)

type MongoMapper struct {
	conn *monc.Model
}

func NewMongoMapper(config *config.Config) *MongoMapper {
	log.Info("NewMongoMapper capnio config: %v, collection: %s", config, CollectionName)
	conn := monc.MustNewModel(config.Mongo.URL, config.Mongo.DB, CollectionName, config.Cache)
	return &MongoMapper{
		conn: conn,
	}
}

func (m *MongoMapper) Insert(ctx context.Context, user *User) error {
	if user.ID.IsZero() {
		user.ID = primitive.NewObjectID()
		user.CreateTime = time.Now()
		user.UpdateTime = user.CreateTime
	}
	_, err := m.conn.InsertOneNoCache(ctx, user)
	return err
}

func (m *MongoMapper) Update(ctx context.Context, user *User) error {
	user.UpdateTime = time.Now()
	_, err := m.conn.UpdateByIDNoCache(ctx, user.ID, bson.M{"$set": user})
	return err
}

func (m *MongoMapper) FindOne(ctx context.Context, id string) (*User, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, consts.ErrInvalidObjectId
	}
	var u User
	err = m.conn.FindOneNoCache(ctx, &u, bson.M{
		consts.ID: oid,
	})
	if err != nil {
		return nil, consts.ErrNotFound
	}
	return &u, nil
}

func (m *MongoMapper) FindOneByPhone(ctx context.Context, phone string) (*User, error) {
	var u User
	err := m.conn.FindOneNoCache(ctx, &u, bson.M{
		consts.Phone: phone,
	})
	switch {
	case err == nil:
		return &u, nil
	case errors.Is(err, monc.ErrNotFound):
		return nil, consts.ErrNotFound
	default:
		return nil, err
	}
}

func (m *MongoMapper) UpdateCount(ctx context.Context, id string, increment int64) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return consts.ErrInvalidObjectId
	}
	_, err = m.conn.UpdateByIDNoCache(ctx, oid, bson.M{
		"$inc": bson.M{
			"count": increment,
		},
	})
	return err
}

// UpdateMbaMemory 更新某用户某 essay_type 下的 memory_summary
func (m *MongoMapper) UpdateMbaMemory(ctx context.Context, id, essayType, memorySummary string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return consts.ErrInvalidObjectId
	}
	_, err = m.conn.UpdateByIDNoCache(ctx, oid, bson.M{
		"$set": bson.M{
			"mba_memory." + essayType: memorySummary,
			"update_time":             time.Now(),
		},
	})
	return err
}

// UpdateVip 叠加一次会员购买时长
func (m *MongoMapper) UpdateVip(ctx context.Context, id string, expireTime time.Time) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return consts.ErrInvalidObjectId
	}
	_, err = m.conn.UpdateByIDNoCache(ctx, oid, bson.M{
		"$set": bson.M{
			"vip_expire_time": expireTime,
			"update_time":     time.Now(),
		},
	})
	return err
}

func (m *MongoMapper) FindUsersNearExpiry(ctx context.Context, expireAfter, expireBefore time.Time) ([]*User, error) {
	var users []*User
	filter := bson.M{
		"vip_expire_time": bson.M{
			"$gt": expireAfter,
			"$lt": expireBefore,
		},
	}
	err := m.conn.Find(ctx, &users, filter, nil)
	if err != nil {
		return nil, err
	}
	return users, nil
}
