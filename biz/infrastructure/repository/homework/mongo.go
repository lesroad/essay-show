package homework

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

const (
	prefixHomeworkCacheKey = "cache:homework"
	HomeworkCollectionName = "homework"
)

type MongoMapper struct {
	conn *monc.Model
}

func NewMongoMapper(config *config.Config) *MongoMapper {
	log.Info("NewHomeworkMongoMapper config: %v, collection: %s", config, HomeworkCollectionName)
	conn := monc.MustNewModel(config.Mongo.URL, config.Mongo.DB, HomeworkCollectionName, config.Cache)
	return &MongoMapper{
		conn: conn,
	}
}

func (m *MongoMapper) Insert(ctx context.Context, homework *Homework) error {
	if homework.ID.IsZero() {
		homework.ID = primitive.NewObjectID()
		homework.CreateTime = time.Now()
		homework.UpdateTime = homework.CreateTime
	}
	_, err := m.conn.InsertOneNoCache(ctx, homework)
	return err
}

func (m *MongoMapper) Update(ctx context.Context, homework *Homework) error {
	homework.UpdateTime = time.Now()
	_, err := m.conn.UpdateByIDNoCache(ctx, homework.ID, bson.M{"$set": homework})
	return err
}

func (m *MongoMapper) FindOne(ctx context.Context, id string) (*Homework, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, consts.ErrInvalidObjectId
	}
	var h Homework
	err = m.conn.FindOneNoCache(ctx, &h, bson.M{
		consts.ID: oid,
	})
	if err != nil {
		return nil, consts.ErrNotFound
	}
	return &h, nil
}

func (m *MongoMapper) FindByCreator(ctx context.Context, creatorID string, page, pageSize int64) ([]*Homework, int64, error) {
	var homeworks []*Homework
	filter := bson.M{"creator_id": creatorID}

	// 获取总数
	total, err := m.conn.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	// 分页查询
	skip := (page - 1) * pageSize
	err = m.conn.Find(ctx, &homeworks, filter, &options.FindOptions{
		Skip:  &skip,
		Limit: &pageSize,
		Sort:  bson.M{"create_time": -1},
	})
	if err != nil {
		return nil, 0, err
	}

	return homeworks, total, nil
}

func (m *MongoMapper) FindByClassID(ctx context.Context, classID string, page, pageSize int64) ([]*Homework, int64, error) {
	var homeworks []*Homework
	filter := bson.M{}
	if classID != "" {
		filter = bson.M{"class_id": classID}
	}

	// 获取总数
	total, err := m.conn.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	// 分页查询
	skip := (page - 1) * pageSize
	err = m.conn.Find(ctx, &homeworks, filter, &options.FindOptions{
		Skip:  &skip,
		Limit: &pageSize,
		Sort:  bson.M{"create_time": -1},
	})
	if err != nil {
		return nil, 0, err
	}

	return homeworks, total, nil
}

func (m *MongoMapper) Delete(ctx context.Context, id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return consts.ErrInvalidObjectId
	}
	_, err = m.conn.DeleteOneNoCache(ctx, bson.M{consts.ID: oid})
	return err
}
