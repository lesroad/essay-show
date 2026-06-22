package mba

import (
	"context"
	"essay-show/biz/application/dto/basic"
	"essay-show/biz/infrastructure/config"
	"essay-show/biz/infrastructure/consts"
	pageutil "essay-show/biz/infrastructure/util/page"

	"github.com/zeromicro/go-zero/core/stores/monc"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MbaQuestionDoc 真题题库文档（直接录入数据库，不通过接口管理）
type MbaQuestionDoc struct {
	ID              primitive.ObjectID `bson:"_id,omitempty"`
	ExamType        int32              `bson:"exam_type"`  // 0=199联考 1=396联考
	TopicType       int32              `bson:"topic_type"` // 0=论证有效性分析 1=论说文
	Year            int32              `bson:"year"`
	EssayType       string             `bson:"essay_type"` // "199_lunxiao" / "199_lunshuo" / "396_lunxiao" / "396_lunshuo"
	Title           string             `bson:"title"`
	Content         string             `bson:"content"` // 题目正文（含材料+作答要求）
	TotalScore      int64              `bson:"total_score"`
	Perspectives    string             `bson:"perspectives"`
}

const (
	questionCollection = "mba_question"
)

// ──────────────────────────────
// 真题题库 Mapper
// ──────────────────────────────

type QuestionMongoMapper struct {
	conn *monc.Model
}

func NewQuestionMongoMapper(cfg *config.Config) *QuestionMongoMapper {
	conn := monc.MustNewModel(cfg.Mongo.URL, cfg.Mongo.DB, questionCollection, cfg.Cache)
	return &QuestionMongoMapper{conn: conn}
}

// FindOne 按 ID 查单道题
func (m *QuestionMongoMapper) FindOne(ctx context.Context, id string) (*MbaQuestionDoc, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, consts.ErrInvalidObjectId
	}
	var q MbaQuestionDoc
	err = m.conn.FindOneNoCache(ctx, &q, bson.M{consts.ID: oid})
	if err != nil {
		return nil, consts.ErrNotFound
	}
	return &q, nil
}

// FindByExamAndTopic 按考试类型 + 题目类型分页查题目列表（按年份倒序）
func (m *QuestionMongoMapper) FindByExamAndTopic(ctx context.Context, examType, topicType int32, p *basic.PaginationOptions) ([]*MbaQuestionDoc, int64, error) {
	skip, limit := pageutil.ParsePageOpt(p)
	filter := bson.M{"exam_type": examType, "topic_type": topicType}

	var docs []*MbaQuestionDoc
	err := m.conn.Find(ctx, &docs, filter, &options.FindOptions{
		Skip:  &skip,
		Limit: &limit,
		Sort:  bson.M{"year": -1},
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := m.conn.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	return docs, total, nil
}

// CountByExamAndTopic 统计某考试类型 + 题目类型的题目总数
func (m *QuestionMongoMapper) CountByExamAndTopic(ctx context.Context, examType, topicType int32) (int64, error) {
	return m.conn.CountDocuments(ctx, bson.M{"exam_type": examType, "topic_type": topicType})
}
