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
	prefixSubmissionCacheKey = "cache:homework_submission"
	SubmissionCollectionName = "homework_submission"
)

type SubmissionMongoMapper struct {
	conn *monc.Model
}

func NewSubmissionMongoMapper(config *config.Config) *SubmissionMongoMapper {
	log.Info("NewSubmissionMongoMapper config: %v, collection: %s", config, SubmissionCollectionName)
	conn := monc.MustNewModel(config.Mongo.URL, config.Mongo.DB, SubmissionCollectionName, config.Cache)
	return &SubmissionMongoMapper{
		conn: conn,
	}
}

func (m *SubmissionMongoMapper) Insert(ctx context.Context, submission *HomeworkSubmission) error {
	if submission.ID.IsZero() {
		submission.ID = primitive.NewObjectID()
		submission.CreateTime = time.Now()
		submission.UpdateTime = time.Now()
	}
	_, err := m.conn.InsertOneNoCache(ctx, submission)
	return err
}

func (m *SubmissionMongoMapper) Update(ctx context.Context, submission *HomeworkSubmission) error {
	submission.UpdateTime = time.Now()
	_, err := m.conn.UpdateByIDNoCache(ctx, submission.ID, bson.M{"$set": submission})
	return err
}

func (m *SubmissionMongoMapper) FindOne(ctx context.Context, id string) (*HomeworkSubmission, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, consts.ErrInvalidObjectId
	}
	var s HomeworkSubmission
	err = m.conn.FindOneNoCache(ctx, &s, bson.M{
		consts.ID: oid,
	})
	if err != nil {
		return nil, consts.ErrNotFound
	}
	return &s, nil
}

func (m *SubmissionMongoMapper) FindByHomeworkID(ctx context.Context, homeworkID string, page, pageSize int64) ([]*HomeworkSubmission, int64, error) {
	var submissions []*HomeworkSubmission
	filter := bson.M{"homework_id": homeworkID}

	// 获取总数
	total, err := m.conn.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	// 分页查询
	skip := (page - 1) * pageSize
	err = m.conn.Find(ctx, &submissions, filter, &options.FindOptions{
		Skip:  &skip,
		Limit: &pageSize,
		Sort:  bson.M{"submit_time": -1},
	})
	if err != nil {
		return nil, 0, err
	}

	return submissions, total, nil
}

func (m *SubmissionMongoMapper) FindByStudentName(ctx context.Context, studentName string, page, pageSize int64) ([]*HomeworkSubmission, int64, error) {
	var submissions []*HomeworkSubmission
	filter := bson.M{"student_name": studentName}

	// 获取总数
	total, err := m.conn.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	// 分页查询
	skip := (page - 1) * pageSize
	err = m.conn.Find(ctx, &submissions, filter, &options.FindOptions{
		Skip:  &skip,
		Limit: &pageSize,
		Sort:  bson.M{"submit_time": -1},
	})
	if err != nil {
		return nil, 0, err
	}

	return submissions, total, nil
}
