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

type Homework struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Subject     int64              `bson:"subject" json:"subject"`
	Topic       int64              `bson:"topic" json:"topic"` // 0.自定义 1.题库
	Title       string             `bson:"title" json:"title"`
	Description string             `bson:"description" json:"description"`
	ClassID     string             `bson:"class_id" json:"classId"`
	Grade       int64              `bson:"grade" json:"grade"`
	TotalScore  int64              `bson:"total_score" json:"totalScore"`
	EssayType   string             `bson:"essay_type" json:"essayType"`
	CreatorID   string             `bson:"creator_id" json:"creatorId"`

	Standard         *string   `bson:"standard,omitempty" json:"standard,omitempty"`
	ContentScore     *int64    `bson:"content_score,omitempty" json:"contentScore,omitempty"`
	ExpressionScore  *int64    `bson:"expression_score,omitempty" json:"expressionScore,omitempty"`
	StructureScore   *int64    `bson:"structure_score,omitempty" json:"structureScore,omitempty"`     // 初中
	DevelopmentScore *int64    `bson:"development_score,omitempty" json:"developmentScore,omitempty"` // 高中
	CreateTime       time.Time `bson:"create_time" json:"createTime"`
	UpdateTime       time.Time `bson:"update_time" json:"updateTime"`
	DeleteTime       time.Time `bson:"delete_time,omitempty" json:"deleteTime"`

	// 网页端提交作业，需自定义批改
	RubricCategories *string `bson:"rubric_categories,omitempty" json:"rubricCategories,omitempty"`
}

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
