package homework

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
	"go.mongodb.org/mongo-driver/mongo"
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

// 根据 homework_id 找所有作业列表，但对每个 student_id 只取最新的一条数据
func (m *SubmissionMongoMapper) FindByHomeworkID(ctx context.Context, homeworkID string) ([]*HomeworkSubmission, error) {
	var submissions []*HomeworkSubmission

	// 使用聚合管道获取每个学生的最新提交记录
	pipeline := []bson.M{
		// 匹配指定作业
		{"$match": bson.M{"homework_id": homeworkID}},
		// 按学生ID分组，获取每个学生的最新提交
		{"$sort": bson.M{"student_id": 1, "create_time": -1}},
		// 按学生ID分组，取每个组的第一条记录（最新的）
		{"$group": bson.M{
			"_id":              "$student_id",
			"latestSubmission": bson.M{"$first": "$$ROOT"},
		}},
		// 替换根文档为最新的提交记录
		{"$replaceRoot": bson.M{"newRoot": "$latestSubmission"}},
		// 按提交时间倒序排列
		{"$sort": bson.M{"create_time": -1}},
	}

	err := m.conn.Aggregate(ctx, &submissions, pipeline)
	if err != nil {
		return nil, err
	}

	return submissions, nil
}

// 查询一条最新的提交记录
func (m *SubmissionMongoMapper) FindByStudentAndHomework(ctx context.Context, studentID, homeworkID string) (*HomeworkSubmission, error) {
	var submission HomeworkSubmission
	filter := bson.M{
		"student_id":  studentID,
		"homework_id": homeworkID,
	}

	err := m.conn.FindOneNoCache(ctx, &submission, filter, &options.FindOneOptions{
		Sort: bson.M{"create_time": -1},
	})
	switch {
	case err == nil:
		return &submission, nil
	case errors.Is(err, mongo.ErrNoDocuments):
		return nil, consts.ErrNotFound
	default:
		return nil, err
	}
}

// FindByStatus 根据状态查找作业提交
func (m *SubmissionMongoMapper) FindByStatus(ctx context.Context, status []int) ([]*HomeworkSubmission, error) {
	var submissions []*HomeworkSubmission
	filter := bson.M{"status": bson.M{"$in": status}}

	err := m.conn.Find(ctx, &submissions, filter, &options.FindOptions{
		Sort: bson.M{"create_time": 1}, // 按创建时间升序，优先处理早提交的
	})
	if err != nil {
		return nil, err
	}

	return submissions, nil
}

// FindTimeoutSubmissions 查找超时的批改任务
func (m *SubmissionMongoMapper) FindTimeoutSubmissions(ctx context.Context, status int, before time.Time) ([]*HomeworkSubmission, error) {
	var submissions []*HomeworkSubmission
	filter := bson.M{
		"status":      status,
		"update_time": bson.M{"$lt": before},
	}

	err := m.conn.Find(ctx, &submissions, filter, &options.FindOptions{
		Sort: bson.M{"update_time": 1}, // 按更新时间升序
	})
	if err != nil {
		return nil, err
	}

	return submissions, nil
}
