package mba

import (
	"context"
	"essay-show/biz/application/dto/basic"
	"essay-show/biz/infrastructure/config"
	"essay-show/biz/infrastructure/consts"
	pageutil "essay-show/biz/infrastructure/util/page"
	"time"

	"github.com/zeromicro/go-zero/core/stores/monc"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MbaRecord 用户 MBA 真题批改记录
type MbaRecord struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"`
	UserId     string             `bson:"user_id"`
	QuestionId string             `bson:"question_id"`
	ExamType   int32              `bson:"exam_type"`  // 0=199联考 1=396联考
	TopicType  int32              `bson:"topic_type"` // 0=论效文 1=论说文
	Year       int32              `bson:"year"`
	EssayType  string             `bson:"essay_type"`
	Ocr        []string           `bson:"ocr"`
	Essay      string             `bson:"essay"`       // 作文原文（ticker worker 重试时使用）
	Status     int32              `bson:"status"`      // 0=初始化 1=批改中 2=已完成 7=失败
	Response   string             `bson:"response"`    // 完整批改结果 JSON
	Score      int64              `bson:"score"`       // 本次得分（从 response 提取）
	TotalScore int64              `bson:"total_score"` // 题目满分（冗余存储，方便统计）
	CreateTime time.Time          `bson:"create_time,omitempty"`
	UpdateTime time.Time          `bson:"update_time,omitempty"`
}

const recordCollection = "mba_record"

// ──────────────────────────────
// 批改记录 Mapper
// ──────────────────────────────

type RecordMongoMapper struct {
	conn *monc.Model
}

func NewRecordMongoMapper(cfg *config.Config) *RecordMongoMapper {
	conn := monc.MustNewModel(cfg.Mongo.URL, cfg.Mongo.DB, recordCollection, cfg.Cache)
	return &RecordMongoMapper{conn: conn}
}

// Insert 插入一条批改记录
func (m *RecordMongoMapper) Insert(ctx context.Context, r *MbaRecord) error {
	if r.ID.IsZero() {
		r.ID = primitive.NewObjectID()
		r.CreateTime = time.Now()
	}
	_, err := m.conn.InsertOne(ctx, r.ID.Hex(), r)
	return err
}

// FindOne 按记录 ID 查单条记录
func (m *RecordMongoMapper) FindOne(ctx context.Context, id string) (*MbaRecord, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, consts.ErrInvalidObjectId
	}
	var r MbaRecord
	err = m.conn.FindOne(ctx, id, &r, bson.M{consts.ID: oid})
	if err != nil {
		return nil, consts.ErrNotFound
	}
	return &r, nil
}

// FindMany 按用户 ID 分页查批改记录（可选按考试类型/题目类型/年份过滤，时间倒序）
func (m *RecordMongoMapper) FindMany(ctx context.Context, userId string, examType, topicType, year *int32, p *basic.PaginationOptions) ([]*MbaRecord, int64, error) {
	skip, limit := pageutil.ParsePageOpt(p)
	filter := bson.M{consts.UserID: userId}
	if examType != nil {
		filter["exam_type"] = *examType
	}
	if topicType != nil {
		filter["topic_type"] = *topicType
	}
	if year != nil {
		filter["year"] = *year
	}

	var records []*MbaRecord
	err := m.conn.Find(ctx, &records, filter, &options.FindOptions{
		Skip:  &skip,
		Limit: &limit,
		Sort:  bson.M{consts.CreateTime: -1},
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := m.conn.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	return records, total, nil
}

// UpdateAfterGrading 批改完成后更新记录（状态 + 结果 + 得分）
func (m *RecordMongoMapper) UpdateAfterGrading(ctx context.Context, id string, status int32, response string, score int64) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return consts.ErrInvalidObjectId
	}
	_, err = m.conn.UpdateByID(ctx, id, oid, bson.M{
		"$set": bson.M{
			"status":      status,
			"response":    response,
			"score":       score,
			"update_time": time.Now(),
		},
	})
	return err
}

// FindByStatus 查询指定状态的批改记录（按创建时间升序）
func (m *RecordMongoMapper) FindByStatus(ctx context.Context, status []int32) ([]*MbaRecord, error) {
	var records []*MbaRecord
	filter := bson.M{"status": bson.M{"$in": status}}
	err := m.conn.Find(ctx, &records, filter, &options.FindOptions{
		Sort: bson.M{consts.CreateTime: 1},
	})
	if err != nil {
		return nil, err
	}
	return records, nil
}

// TryUpdateStatusToGrading CAS 更新：仅当 status==fromStatus 时才更新为 toStatus，防止重复消费
func (m *RecordMongoMapper) TryUpdateStatusToGrading(ctx context.Context, id primitive.ObjectID, fromStatus, toStatus int32) (bool, error) {
	filter := bson.M{
		"_id":    id,
		"status": fromStatus,
	}
	update := bson.M{
		"$set": bson.M{
			"status":      toStatus,
			"update_time": time.Now(),
		},
	}
	result, err := m.conn.UpdateOneNoCache(ctx, filter, update)
	if err != nil {
		return false, err
	}
	return result.MatchedCount > 0, nil
}

// FindTimeoutRecords 查询超时的批改任务（update_time 早于 before 且 status 匹配）
func (m *RecordMongoMapper) FindTimeoutRecords(ctx context.Context, status int32, before time.Time) ([]*MbaRecord, error) {
	var records []*MbaRecord
	filter := bson.M{
		"status":      status,
		"update_time": bson.M{"$lt": before},
	}
	err := m.conn.Find(ctx, &records, filter, &options.FindOptions{
		Sort: bson.M{"update_time": 1},
	})
	if err != nil {
		return nil, err
	}
	return records, nil
}

// FindLatestByQuestionId 查某用户某题目最近一次批改记录
func (m *RecordMongoMapper) FindLatestByQuestionId(ctx context.Context, userId, questionId string) (*MbaRecord, error) {
	filter := bson.M{consts.UserID: userId, "question_id": questionId}
	var r MbaRecord
	err := m.conn.FindOneNoCache(ctx, &r, filter, &options.FindOneOptions{
		Sort: bson.M{consts.CreateTime: -1},
	})
	if err != nil {
		return nil, consts.ErrNotFound
	}
	return &r, nil
}

// CountCompletedByExamAndTopic 统计某用户某考试类型+题目类型已完成的题目数（distinct questionId）
func (m *RecordMongoMapper) CountCompletedByExamAndTopic(ctx context.Context, userId string, examType, topicType int32) (int64, error) {
	pipeline := bson.A{
		bson.M{"$match": bson.M{consts.UserID: userId, "exam_type": examType, "topic_type": topicType}},
		bson.M{"$group": bson.M{"_id": "$question_id"}},
		bson.M{"$count": "count"},
	}
	var result []struct {
		Count int64 `bson:"count"`
	}
	if err := m.conn.Aggregate(ctx, &result, pipeline); err != nil || len(result) == 0 {
		return 0, nil
	}
	return result[0].Count, nil
}

// AverageScoreRateByExamAndTopic 统计某用户某考试类型+题目类型的平均得分率
func (m *RecordMongoMapper) AverageScoreRateByExamAndTopic(ctx context.Context, userId string, examType, topicType int32) (float64, error) {
	pipeline := bson.A{
		bson.M{"$match": bson.M{consts.UserID: userId, "exam_type": examType, "topic_type": topicType}},
		bson.M{"$sort": bson.M{consts.CreateTime: -1}},
		bson.M{"$group": bson.M{
			"_id":        "$question_id",
			"score":      bson.M{"$first": "$score"},
			"totalScore": bson.M{"$first": "$total_score"},
		}},
		bson.M{"$project": bson.M{
			"rate": bson.M{"$cond": bson.A{
				bson.M{"$gt": bson.A{"$totalScore", 0}},
				bson.M{"$multiply": bson.A{bson.M{"$divide": bson.A{"$score", "$totalScore"}}, 100}},
				0,
			}},
		}},
		bson.M{"$group": bson.M{"_id": nil, "avgRate": bson.M{"$avg": "$rate"}}},
	}
	var result []struct {
		AvgRate float64 `bson:"avgRate"`
	}
	if err := m.conn.Aggregate(ctx, &result, pipeline); err != nil || len(result) == 0 {
		return 0, nil
	}
	return result[0].AvgRate, nil
}
