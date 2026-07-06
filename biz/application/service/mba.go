package service

import (
	"context"
	"encoding/json"
	"essay-show/biz/adaptor"
	"essay-show/biz/application/dto/essay/show"
	"essay-show/biz/infrastructure/consts"
	mbaRepo "essay-show/biz/infrastructure/repository/mba"
	"essay-show/biz/infrastructure/repository/user"
	"essay-show/biz/infrastructure/util"
	logx "essay-show/biz/infrastructure/util/log"
	"sync"
	"time"

	"github.com/google/wire"
	"github.com/spf13/cast"
)

type IMbaService interface {
	ListMbaQuestions(ctx context.Context, req *show.ListMbaQuestionsReq) (*show.ListMbaQuestionsResp, error)
	GetMbaQuestion(ctx context.Context, req *show.GetMbaQuestionReq) (*show.GetMbaQuestionResp, error)
	SubmitMbaAnswer(ctx context.Context, req *show.SubmitMbaAnswerReq) (*show.SubmitMbaAnswerResp, error)
	GetMbaEvaluate(ctx context.Context, req *show.GetMbaEvaluateReq) (*show.GetMbaEvaluateResp, error)
	ListMbaEvaluates(ctx context.Context, req *show.ListMbaEvaluatesReq) (*show.ListMbaEvaluatesResp, error)
	StartGrader(ctx context.Context) error
}

type MbaService struct {
	QuestionMapper *mbaRepo.QuestionMongoMapper
	RecordMapper   *mbaRepo.RecordMongoMapper
	UserMapper     *user.MongoMapper
}

var MbaServiceSet = wire.NewSet(
	wire.Struct(new(MbaService), "*"),
	wire.Bind(new(IMbaService), new(*MbaService)),
)

// checkMbaAccess 校验用户是否持有任意 MBA 考试资格（exam_199 或 exam_396 均可访问全部题目）。
func (s *MbaService) checkMbaAccess(ctx context.Context, userId string) error {
	u, err := s.UserMapper.FindOne(ctx, userId)
	if err != nil {
		return consts.ErrNotFound
	}
	if u.Role != consts.Role199th && u.Role != consts.Role396th {
		return consts.ErrNotAuthentication
	}
	return nil
}

// ListMbaQuestions 获取某考试类型+题目类型的真题列表（含是否已作答）
func (s *MbaService) ListMbaQuestions(ctx context.Context, req *show.ListMbaQuestionsReq) (*show.ListMbaQuestionsResp, error) {
	meta := adaptor.ExtractUserMeta(ctx)
	if meta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}
	if err := s.checkMbaAccess(ctx, meta.GetUserId()); err != nil {
		return nil, err
	}

	docs, total, err := s.QuestionMapper.FindByExamAndTopic(ctx, int32(req.ExamType), int32(req.TopicType), req.PaginationOptions)
	if err != nil {
		return nil, consts.ErrNotFound
	}

	var answeredCount int64
	var scoreSum, totalScoreSum float64

	briefs := make([]*show.MbaQuestionBrief, 0, len(docs))
	for _, doc := range docs {
		brief := &show.MbaQuestionBrief{
			Id:        doc.ID.Hex(),
			ExamType:  show.MbaExamType(doc.ExamType),
			TopicType: show.MbaTopicType(doc.TopicType),
			Year:      doc.Year,
			Title:     doc.Title,
		}
		if record, err := s.RecordMapper.FindLatestByQuestionId(ctx, meta.GetUserId(), doc.ID.Hex()); err == nil {
			brief.HasAnswered = true
			evaluateId := record.ID.Hex()
			brief.LatestEvaluateId = &evaluateId
			if record.Status == consts.StatusCompleted {
				brief.Score = &record.Score
				answeredCount++
				scoreSum += float64(record.Score)
				totalScoreSum += float64(record.TotalScore)
			}
		}
		briefs = append(briefs, brief)
	}

	var scoreRate float64
	if totalScoreSum > 0 {
		scoreRate = scoreSum / totalScoreSum * 100
	}

	return &show.ListMbaQuestionsResp{
		Questions:     briefs,
		Total:         total,
		AnsweredCount: answeredCount,
		ScoreRate:     scoreRate,
	}, nil
}

// GetMbaQuestion 获取真题详情
func (s *MbaService) GetMbaQuestion(ctx context.Context, req *show.GetMbaQuestionReq) (*show.GetMbaQuestionResp, error) {
	meta := adaptor.ExtractUserMeta(ctx)
	if meta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}

	doc, err := s.QuestionMapper.FindOne(ctx, req.QuestionId)
	if err != nil {
		return nil, consts.ErrNotFound
	}
	if err := s.checkMbaAccess(ctx, meta.GetUserId()); err != nil {
		return nil, err
	}

	return &show.GetMbaQuestionResp{
		Question: &show.MbaQuestion{
			Id:           doc.ID.Hex(),
			ExamType:     show.MbaExamType(doc.ExamType),
			TopicType:    show.MbaTopicType(doc.TopicType),
			Year:         doc.Year,
			Content:      doc.Content,
			TotalScore:   doc.TotalScore,
			Perspectives: doc.Perspectives,
		},
	}, nil
}

// SubmitMbaAnswer 提交作文。立即写入 StatusInitialized 记录并返回 evaluateId，
// 批改由 StartGrader 后台 ticker 负责，重启安全。
func (s *MbaService) SubmitMbaAnswer(ctx context.Context, req *show.SubmitMbaAnswerReq) (*show.SubmitMbaAnswerResp, error) {
	meta := adaptor.ExtractUserMeta(ctx)
	if meta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}

	question, err := s.QuestionMapper.FindOne(ctx, req.QuestionId)
	if err != nil {
		return nil, consts.ErrNotFound
	}
	if err := s.checkMbaAccess(ctx, meta.GetUserId()); err != nil {
		return nil, err
	}

	record := &mbaRepo.MbaRecord{
		UserId:     meta.GetUserId(),
		QuestionId: req.QuestionId,
		ExamType:   question.ExamType,
		TopicType:  question.TopicType,
		Year:       question.Year,
		EssayType:  question.EssayType,
		Title:      req.Title,
		Ocr:        req.Ocr,
		Essay:      req.Text,
		Status:     consts.StatusInitialized,
		TotalScore: question.TotalScore,
	}
	if record.Title == "" {
		record.Title = question.Title
	}
	if err := s.RecordMapper.Insert(ctx, record); err != nil {
		logx.Error("MbaRecord Insert error: %v", err)
		return nil, consts.ErrCall
	}

	return &show.SubmitMbaAnswerResp{
		EvaluateId: record.ID.Hex(),
	}, nil
}

// GetMbaEvaluate 查询批改结果（含状态）
func (s *MbaService) GetMbaEvaluate(ctx context.Context, req *show.GetMbaEvaluateReq) (*show.GetMbaEvaluateResp, error) {
	meta := adaptor.ExtractUserMeta(ctx)
	if meta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}

	record, err := s.RecordMapper.FindOne(ctx, req.EvaluateId)
	if err != nil {
		return nil, consts.ErrNotFound
	}
	if err := s.checkMbaAccess(ctx, meta.GetUserId()); err != nil {
		return nil, err
	}

	title := record.Title
	if title == "" {
		if question, err := s.QuestionMapper.FindOne(ctx, record.QuestionId); err == nil {
			title = question.Title
		}
	}

	return &show.GetMbaEvaluateResp{
		Id:         record.ID.Hex(),
		QuestionId: record.QuestionId,
		ExamType:   show.MbaExamType(record.ExamType),
		TopicType:  show.MbaTopicType(record.TopicType),
		Year:       record.Year,
		Title:      title,
		Status:     record.Status,
		Response:   record.Response,
		Score:      record.Score,
		CreateTime: record.CreateTime.Unix(),
	}, nil
}

// ListMbaEvaluates 获取当前用户的批改记录列表（支持按年份/考试类型/题目类型过滤）
func (s *MbaService) ListMbaEvaluates(ctx context.Context, req *show.ListMbaEvaluatesReq) (*show.ListMbaEvaluatesResp, error) {
	meta := adaptor.ExtractUserMeta(ctx)
	if meta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}
	if err := s.checkMbaAccess(ctx, meta.GetUserId()); err != nil {
		return nil, err
	}

	var examType, topicType, year *int32
	if req.ExamType != nil {
		v := int32(*req.ExamType)
		examType = &v
	}
	if req.TopicType != nil {
		v := int32(*req.TopicType)
		topicType = &v
	}
	if req.Year != nil {
		year = req.Year
	}

	records, total, err := s.RecordMapper.FindMany(ctx, meta.GetUserId(), examType, topicType, year, req.PaginationOptions)
	if err != nil {
		return nil, consts.ErrNotFound
	}

	briefs := make([]*show.MbaEvaluateBrief, 0, len(records))
	for _, r := range records {
		briefs = append(briefs, &show.MbaEvaluateBrief{
			Id:         r.ID.Hex(),
			QuestionId: r.QuestionId,
			ExamType:   show.MbaExamType(r.ExamType),
			TopicType:  show.MbaTopicType(r.TopicType),
			Year:       r.Year,
			Status:     r.Status,
			Score:      r.Score,
			TotalScore: r.TotalScore,
			CreateTime: r.CreateTime.Unix(),
		})
	}

	return &show.ListMbaEvaluatesResp{
		Evaluates: briefs,
		Total:     total,
	}, nil
}

// ──────────────────────────────────────────────────────────────────
// 后台批改 Ticker（与作业模块保持一致的 polling 模式）
// ──────────────────────────────────────────────────────────────────

const mbaConcurrency = 5

// StartGrader 启动 MBA 批改定时器（服务启动时调用一次）
func (s *MbaService) StartGrader(ctx context.Context) error {
	logx.Info("启动 MBA 批改定时器")
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.processMbaRecords(context.Background())
			}
		}
	}()
	return nil
}

// processMbaRecords 扫描 StatusInitialized 记录并并发批改
func (s *MbaService) processMbaRecords(ctx context.Context) {
	defer s.processTimeoutMbaRecords(ctx)

	records, err := s.RecordMapper.FindByStatus(ctx, []int32{consts.StatusInitialized})
	if err != nil {
		logx.Error("processMbaRecords FindByStatus error: %v", err)
		return
	}
	if len(records) == 0 {
		return
	}

	logx.Info("processMbaRecords: 发现 %d 条待批改记录", len(records))

	sem := make(chan struct{}, mbaConcurrency)
	var wg sync.WaitGroup

	for _, r := range records {
		ok, err := s.RecordMapper.TryUpdateStatusToGrading(ctx, r.ID, consts.StatusInitialized, consts.StatusGrading)
		if err != nil {
			logx.Error("processMbaRecords TryUpdateStatusToGrading error: %v", err)
			continue
		}
		if !ok {
			continue
		}

		sem <- struct{}{}
		wg.Add(1)
		go func(rec *mbaRepo.MbaRecord) {
			defer func() {
				<-sem
				wg.Done()
			}()
			s.processOneRecord(context.Background(), rec)
		}(r)
	}
	wg.Wait()
}

// processTimeoutMbaRecords 将超过 20 分钟仍处于 StatusGrading 的记录重置为 StatusInitialized
func (s *MbaService) processTimeoutMbaRecords(ctx context.Context) {
	timeoutTime := time.Now().Add(-20 * time.Minute)
	records, err := s.RecordMapper.FindTimeoutRecords(ctx, consts.StatusGrading, timeoutTime)
	if err != nil {
		return
	}
	for _, r := range records {
		if err := s.RecordMapper.UpdateAfterGrading(ctx, r.ID.Hex(), consts.StatusInitialized, "", 0); err != nil {
			logx.Error("processTimeoutMbaRecords reset error: %v, recordId: %s", err, r.ID.Hex())
			continue
		}
		logx.Info("processTimeoutMbaRecords: 重置超时任务 %s", r.ID.Hex())
	}
}

// processOneRecord 加载题目和 memory，按需调 OCR，然后驱动 runGrading 执行批改
func (s *MbaService) processOneRecord(ctx context.Context, r *mbaRepo.MbaRecord) {
	// 1. 确定作文原文：优先 Essay 字段（直接提交），否则从 Ocr 图片识别
	essay := r.Essay
	if essay == "" && len(r.Ocr) > 0 {
		_, content, err := util.GetHttpClient().OcrExtract(ctx, r.Ocr)
		if err != nil {
			logx.Error("processOneRecord OcrExtract error: %v, recordId: %s", err, r.ID.Hex())
			_ = s.RecordMapper.UpdateAfterGrading(ctx, r.ID.Hex(), consts.StatusFailed, "", 0)
			return
		}
		essay = content
	}
	if essay == "" {
		logx.Error("processOneRecord: 作文内容为空, recordId: %s", r.ID.Hex())
		_ = s.RecordMapper.UpdateAfterGrading(ctx, r.ID.Hex(), consts.StatusFailed, "", 0)
		return
	}

	// 2. 加载题目
	question, err := s.QuestionMapper.FindOne(ctx, r.QuestionId)
	if err != nil {
		logx.Error("processOneRecord FindQuestion error: %v, recordId: %s", err, r.ID.Hex())
		_ = s.RecordMapper.UpdateAfterGrading(ctx, r.ID.Hex(), consts.StatusFailed, "", 0)
		return
	}

	// 3. 读取 memory_summary
	memorySummary := ""
	if u, err := s.UserMapper.FindOne(ctx, r.UserId); err == nil && u.MbaMemory != nil {
		memorySummary = u.MbaMemory[r.EssayType]
	}

	s.runGrading(ctx, r.ID.Hex(), r.UserId, r.EssayType, question.Content, question.Perspectives, essay, memorySummary)
}

// runGrading 调用 AI 批改接口并将结果写回数据库
func (s *MbaService) runGrading(ctx context.Context, recordId, userId, essayType, material, perspectives, essay, memorySummary string) {
	client := util.GetHttpClient()
	raw, err := client.MbaGrade(ctx, essayType, material, perspectives, essay, memorySummary)
	if err != nil {
		logx.Error("runGrading MbaGrade error: %v, recordId: %s", err, recordId)
		_ = s.RecordMapper.UpdateAfterGrading(ctx, recordId, consts.StatusFailed, "", 0)
		return
	}

	inner, _ := raw["result"].(map[string]any)
	if inner == nil {
		logx.Error("runGrading: missing result field, recordId: %s", recordId)
		_ = s.RecordMapper.UpdateAfterGrading(ctx, recordId, consts.StatusFailed, "", 0)
		return
	}

	responseBytes, _ := json.Marshal(inner)
	responseStr := string(responseBytes)
	score := cast.ToInt64(inner["total_score"])

	if err := s.RecordMapper.UpdateAfterGrading(ctx, recordId, consts.StatusCompleted, responseStr, score); err != nil {
		logx.Error("runGrading UpdateAfterGrading error: %v, recordId: %s", err, recordId)
	}

	// updated_summary 是下次批改要带的 memory_summary
	newMemory := cast.ToString(inner["updated_summary"])
	if newMemory != "" {
		if err := s.UserMapper.UpdateMbaMemory(ctx, userId, essayType, newMemory); err != nil {
			logx.Error("runGrading UpdateMbaMemory error: %v", err)
		}
	}
}
