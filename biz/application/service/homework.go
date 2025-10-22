package service

import (
	"context"
	"encoding/json"
	"essay-show/biz/adaptor"
	"essay-show/biz/application/dto/essay/show"
	"essay-show/biz/application/dto/essay/stateless"
	"essay-show/biz/infrastructure/consts"
	"essay-show/biz/infrastructure/repository/class"
	"essay-show/biz/infrastructure/repository/homework"
	"essay-show/biz/infrastructure/repository/user"
	"essay-show/biz/infrastructure/util"
	"essay-show/biz/infrastructure/util/log"
	"strings"
	"time"

	"github.com/google/wire"
)

type IHomeworkService interface {
	CreateHomework(ctx context.Context, req *show.CreateHomeworkReq) (*show.CreateHomeworkResp, error)
	ListHomeworks(ctx context.Context, req *show.ListHomeworksReq) (*show.ListHomeworksResp, error)
	SubmitHomework(ctx context.Context, req *show.SubmitHomeworkReq) (*show.SubmitHomeworkResp, error)
	GetSubmissions(ctx context.Context, req *show.GetSubmissionsReq) (*show.GetSubmissionsResp, error)
	GetSubmissionEvaluate(ctx context.Context, req *show.GetSubmissionEvaluateReq) (*show.GetSubmissionEvaluateResp, error)
	StartGrader(ctx context.Context) error
}

type HomeworkService struct {
	HomeworkMapper   *homework.MongoMapper
	SubmissionMapper *homework.SubmissionMongoMapper
	ClassMapper      *class.MongoMapper
	MemberMapper     *class.MemberMongoMapper
	UserMapper       *user.MongoMapper
	EssayService     IEssayService
}

var HomeworkServiceSet = wire.NewSet(
	wire.Struct(new(HomeworkService), "*"),
	wire.Bind(new(IHomeworkService), new(*HomeworkService)),
)

// CreateHomework 创建作业
func (s *HomeworkService) CreateHomework(ctx context.Context, req *show.CreateHomeworkReq) (*show.CreateHomeworkResp, error) {
	// 获取用户信息
	userMeta := adaptor.ExtractUserMeta(ctx)
	if userMeta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}

	// 校验教师身份
	user, err := s.UserMapper.FindOne(ctx, userMeta.GetUserId())
	if err != nil {
		log.Error("获取用户信息失败: %v", err)
		return nil, consts.ErrNotFound
	}
	if user.Role != consts.RoleTeacher {
		return nil, consts.ErrNotAuthentication
	}

	// 验证班级是否存在
	_, err = s.ClassMapper.FindOne(ctx, req.ClassId)
	if err != nil {
		log.Error("班级不存在: %v", err)
		return nil, consts.ErrNotFound
	}

	// 创建作业
	now := time.Now()
	h := &homework.Homework{
		Title:       req.Title,
		Description: req.Description,
		ClassID:     req.ClassId,
		Grade:       req.Grade,
		TotalScore:  req.TotalScore,
		EssayType:   req.EssayType,
		CreatorID:   userMeta.GetUserId(),
		CreateTime:  now,
		UpdateTime:  now,
	}

	err = s.HomeworkMapper.Insert(ctx, h)
	if err != nil {
		log.Error("创建作业失败: %v", err)
		return nil, consts.ErrCreateHomework
	}

	return &show.CreateHomeworkResp{
		HomeworkId: h.ID.Hex(),
	}, nil
}

// ListHomeworks 获取作业列表
func (s *HomeworkService) ListHomeworks(ctx context.Context, req *show.ListHomeworksReq) (*show.ListHomeworksResp, error) {
	// 获取用户信息
	userMeta := adaptor.ExtractUserMeta(ctx)
	if userMeta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}

	// 确认身份
	u, err := s.UserMapper.FindOne(ctx, userMeta.GetUserId())
	if err != nil {
		log.Error("获取用户信息失败: %v", err)
		return nil, consts.ErrNotFound
	}

	// 老师检查是否为班级创建者，学生检查是否加入班级
	c := new(class.Class)
	if u.Role == consts.RoleTeacher {
		c, err = s.ClassMapper.FindOne(ctx, req.ClassId)
		if err != nil {
			log.Error("班级不存在: %v", err)
			return nil, consts.ErrNotFound
		}
		if c.CreatorID != userMeta.GetUserId() {
			return nil, consts.ErrForbidden
		}
	} else {
		_, err = s.MemberMapper.FindByClassIDAndStuID(ctx, req.ClassId, userMeta.GetUserId())
		if err == consts.ErrNotFound {
			log.Error("用户不是班级成员")
			return nil, consts.ErrNotClassMember
		}
	}

	page := int64(1)
	pageSize := int64(50)
	if req.PaginationOptions != nil {
		if req.PaginationOptions.Page != nil {
			page = *req.PaginationOptions.Page
		}
		if req.PaginationOptions.Limit != nil {
			pageSize = *req.PaginationOptions.Limit
		}
	}

	homeworks, total, err := s.HomeworkMapper.FindByClassID(ctx, req.ClassId, page, pageSize)
	if err != nil {
		log.Error("获取作业列表失败: %v", err)
		return nil, consts.ErrGetHomeworkList
	}

	homeworkInfos := make([]*show.HomeworkInfo, 0, len(homeworks))
	for _, h := range homeworks {
		homeworkInfo := &show.HomeworkInfo{
			Id:          h.ID.Hex(),
			Title:       h.Title,
			Description: h.Description,
			TotalScore:  h.TotalScore,
			EssayType:   h.EssayType,
			CreateTime:  h.CreateTime.Unix(),
		}

		if u.Role == consts.RoleTeacher {
			// 获取提交数量
			homeworkInfos, err := s.SubmissionMapper.FindByHomeworkID(ctx, h.ID.Hex())
			if err != nil {
				log.Error("获取提交情况失败: %v", err)
				return nil, consts.ErrGetHomeworkList
			}
			submitCount := int64(len(homeworkInfos))

			// 计算未提交学生数
			notSubmittedCount := c.MemberCount - submitCount - 1

			// 获取已批改数量
			gradeList, err := s.SubmissionMapper.FindByStatus(ctx, consts.StatusCompleted)
			if err != nil {
				log.Error("获取已批改数量失败: %v", err)
				return nil, consts.ErrGetHomeworkList
			}
			gradeCount := int64(len(gradeList))

			homeworkInfo.SubmissionCount = &submitCount
			homeworkInfo.NotSubmittedCount = &notSubmittedCount
			homeworkInfo.GradeCount = &gradeCount
		} else {
			// 获取提交状态
			submission, err := s.SubmissionMapper.FindByStudentAndHomework(ctx, userMeta.GetUserId(), h.ID.Hex())
			switch {
			case err == consts.ErrNotFound:
				status := show.HomeworkStatus(consts.StatusNotSubmission)
				homeworkInfo.Status = &status
			case err != nil:
				log.Error("获取提交情况失败: %v", err)
				return nil, consts.ErrGetHomeworkList
			default:
				status := show.HomeworkStatus(submission.Status)
				submissionId := submission.ID.Hex()
				submitTime := submission.CreateTime.Unix()

				homeworkInfo.Status = &status
				homeworkInfo.SubmissionId = &submissionId
				homeworkInfo.SubmitTime = &submitTime

				if submission.Status == int(consts.StatusCompleted) {
					homeworkInfo.GradeResult = &submission.GradeResult
				}
			}
		}
		homeworkInfos = append(homeworkInfos, homeworkInfo)
	}

	return &show.ListHomeworksResp{
		Homeworks: homeworkInfos,
		Total:     total,
	}, nil
}

// GetHomework 获取作业批改结果
func (s *HomeworkService) GetSubmissionEvaluate(ctx context.Context, req *show.GetSubmissionEvaluateReq) (*show.GetSubmissionEvaluateResp, error) {
	// 获取用户信息
	userMeta := adaptor.ExtractUserMeta(ctx)
	if userMeta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}

	// 获取提交情况
	submission, err := s.SubmissionMapper.FindOne(ctx, req.SubmissionId)
	if err != nil {
		log.Error("获取作业详情失败: %v", err)
		return nil, consts.ErrGetHomework
	}

	if submission.Status != consts.StatusCompleted {
		log.Error("批改未完成")
		return nil, consts.ErrHomeworkNotGrade
	}

	return &show.GetSubmissionEvaluateResp{
		Id:       submission.ID.Hex(),
		Response: submission.Response,
	}, nil
}

// SubmitHomework 学生提交作业
func (s *HomeworkService) SubmitHomework(ctx context.Context, req *show.SubmitHomeworkReq) (*show.SubmitHomeworkResp, error) {
	userMeta := adaptor.ExtractUserMeta(ctx)
	if userMeta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}

	// 确认学生身份
	u, err := s.UserMapper.FindOne(ctx, userMeta.GetUserId())
	if err != nil {
		log.Error("获取用户信息失败: %v", err)
		return nil, consts.ErrNotFound
	}
	if u.Role != consts.RoleStudent {
		return nil, consts.ErrNotAuthentication
	}

	h, err := s.HomeworkMapper.FindOne(ctx, req.HomeworkId)
	if err != nil {
		log.Error("作业不存在: %v", err)
		return nil, consts.ErrNotFound
	}

	now := time.Now()
	submission := &homework.HomeworkSubmission{
		HomeworkID: req.HomeworkId,
		StudentID:  userMeta.UserId,
		TeacherID:  h.CreatorID,
		Images:     req.Images,
		Status:     consts.StatusInitialized,
		CreateTime: now,
		UpdateTime: now,
	}

	err = s.SubmissionMapper.Insert(ctx, submission)
	if err != nil {
		log.Error("提交作业失败: %v", err)
		return nil, consts.ErrSubmitHomework
	}

	log.Info("作业提交成功 [SubmissionID: %s, StudentID: %s, HomeworkID: %s]",
		submission.ID.Hex(), userMeta.UserId, req.HomeworkId)

	return &show.SubmitHomeworkResp{
		SubmissionId: submission.ID.Hex(),
	}, nil
}

// GetSubmissions 教师端获取提交详情
func (s *HomeworkService) GetSubmissions(ctx context.Context, req *show.GetSubmissionsReq) (*show.GetSubmissionsResp, error) {
	// 获取用户信息
	userMeta := adaptor.ExtractUserMeta(ctx)
	if userMeta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}

	// 解析分页参数
	page := int64(1)
	pageSize := int64(10)
	if req.PaginationOptions != nil {
		if req.PaginationOptions.Page != nil {
			page = *req.PaginationOptions.Page
		}
		if req.PaginationOptions.Limit != nil {
			pageSize = *req.PaginationOptions.Limit
		}
	}

	// 确认老师身份
	u, err := s.UserMapper.FindOne(ctx, userMeta.GetUserId())
	if err != nil {
		log.Error("获取用户信息失败: %v", err)
		return nil, consts.ErrNotFound
	}
	if u.Role != consts.RoleTeacher {
		return nil, consts.ErrNotAuthentication
	}

	// 获取作业信息
	h, err := s.HomeworkMapper.FindOne(ctx, req.HomeworkId)
	if err != nil {
		log.Error("作业不存在: %v", err)
		return nil, consts.ErrNotFound
	}

	// 获取班级成员
	members, total, err := s.MemberMapper.FindByClassID(ctx, h.ClassID, page, pageSize)
	if err != nil {
		log.Error("获取班级成员失败: %v", err)
		return nil, consts.ErrGetClassMembers
	}

	submissionInfos := make([]*show.SubmissionInfo, 0)
	for _, m := range members {
		// 如果加入班级时是老师，就无需提交
		if m.Role == consts.RoleTeacher {
			total = total - 1
			continue
		}

		// 获取学生信息
		user, err := s.UserMapper.FindOne(ctx, m.UserID)
		if err != nil {
			log.Error("获取学生信息失败: %v", err)
			return nil, consts.ErrGetSubmission
		}

		sub := &show.SubmissionInfo{StudentName: user.Username}

		// 查询学生提交记录
		userSubmission, err := s.SubmissionMapper.FindByStudentAndHomework(ctx, user.ID.Hex(), req.HomeworkId)
		switch {
		case err == consts.ErrNotFound:
			sub.Status = consts.StatusNotSubmission
		case err != nil:
			log.Error("获取学生提交记录失败: %v", err)
			return nil, consts.ErrGetSubmission
		default:
			sub.Status = show.HomeworkStatus(userSubmission.Status)
			id := userSubmission.ID.Hex()
			submitTime := userSubmission.CreateTime.Unix()

			sub.Id = &id
			sub.Title = &userSubmission.Title
			sub.SubmitTime = &submitTime
			if userSubmission.Status == consts.StatusCompleted {
				sub.GradeResult = &userSubmission.GradeResult
			}
		}

		submissionInfos = append(submissionInfos, sub)
	}

	return &show.GetSubmissionsResp{
		Submissions: submissionInfos,
		Total:       total,
	}, nil
}

// StartGrader 启动作业批改定时器
func (s *HomeworkService) StartGrader(ctx context.Context) error {
	log.Info("启动作业批改定时器")

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.processHomeworkSubmissions(context.Background())
			}
		}
	}()

	return nil
}

// processHomeworkSubmissions 处理待批改的作业
func (s *HomeworkService) processHomeworkSubmissions(ctx context.Context) {
	submissions, err := s.SubmissionMapper.FindByStatus(ctx, consts.StatusInitialized)
	if err != nil {
		log.Error("查询待批改作业失败: %v", err)
		return
	}

	if len(submissions) == 0 {
		return
	}

	log.Info("找到 %d 个待批改的作业", len(submissions))

	for _, submission := range submissions {
		log.Info("作业详情:%+v", submission)
		s.processOneSubmission(ctx, submission)
	}

	// 处理超时任务
	s.processTimeoutSubmissions(ctx)
}

// processOneSubmission 处理单个作业提交
func (s *HomeworkService) processOneSubmission(ctx context.Context, submission *homework.HomeworkSubmission) {
	// 查询老师批改次数
	teacher, err := s.UserMapper.FindOne(ctx, submission.TeacherID)
	if err != nil {
		log.Error("查询老师信息失败: %v", err)
		markSubmissionFailed(ctx, submission, s.SubmissionMapper, err.Error())
		return
	}
	if teacher.Count < 1 {
		markSubmissionFailed(ctx, submission, s.SubmissionMapper, "老师批改次数不足")
		return
	}

	// 获取作业
	homework, err := s.HomeworkMapper.FindOne(ctx, submission.HomeworkID)
	if err != nil {
		markSubmissionFailed(ctx, submission, s.SubmissionMapper, "作业不存在")
		return
	}

	// OCR识别
	client := util.GetHttpClient()
	ocrResp, err := client.TitleUrlOCR(ctx, submission.Images, "")
	if err != nil {
		markSubmissionFailed(ctx, submission, s.SubmissionMapper, err.Error())
		return
	}

	if ocrResp["code"].(float64) != 0 {
		markSubmissionFailed(ctx, submission, s.SubmissionMapper, "OCR失败")
		return
	}

	// 解析结果
	data := ocrResp["data"].(map[string]any)
	title := data["title"].(string)
	text := data["content"].(string)
	prompt := homework.Description
	essayType := homework.EssayType
	grade := homework.Grade
	totalScore := homework.TotalScore

	// 更新为批改中
	submission.Status = consts.StatusGrading
	submission.UpdateTime = time.Now()
	submission.Title = title
	s.SubmissionMapper.Update(ctx, submission)

	resultChan := make(chan string, 100)
	var finalResult string

	// 调用批改服务
	go func() {
		defer close(resultChan)
		client.EvaluateStream(ctx, title, text, &grade, &totalScore, &essayType, &prompt, resultChan)
	}()

	for jsonMessage := range resultChan {
		var data map[string]any
		if parseErr := json.Unmarshal([]byte(jsonMessage), &data); parseErr != nil {
			log.Error("解析下游JSON消息失败: %v", parseErr)
			continue
		}
		// 检查消息类型并转发
		if msgType, ok := data["type"].(string); ok {
			switch msgType {
			case "complete":
				if result, ok := data["data"].(map[string]interface{}); ok {
					if resultBytes, err := json.Marshal(result); err == nil {
						finalResult = string(resultBytes)
					}
				}
			case "error":
				markSubmissionFailed(ctx, submission, s.SubmissionMapper, data["message"].(string))
				return
			default:
			}
		}
	}

	if len(finalResult) == 0 {
		markSubmissionFailed(ctx, submission, s.SubmissionMapper, "批改结果为空")
		return
	}

	// 解析存储的批改结果到结构体
	var evaluateResult stateless.Evaluate
	if err := json.Unmarshal([]byte(finalResult), &evaluateResult); err != nil {
		markSubmissionFailed(ctx, submission, s.SubmissionMapper, "批改结果不合法")
		return
	}

	// 扣除老师批改次数
	if err := s.UserMapper.UpdateCount(ctx, submission.TeacherID, -1); err != nil {
		markSubmissionFailed(ctx, submission, s.SubmissionMapper, "扣除批改次数失败")
		log.Error("扣除老师批改次数失败: %v", err)
		return
	}

	// 保存批改结果
	submission.Status = consts.StatusCompleted
	submission.UpdateTime = time.Now()
	submission.Response = finalResult
	submission.GradeResult = strings.Split(evaluateResult.AIEvaluation.ScoreEvaluation.Scores.AllWithTotal, "/")[0]
	if err := s.SubmissionMapper.Update(ctx, submission); err != nil {
		log.Error("保存批改结果失败: %v", err)
		markSubmissionFailed(ctx, submission, s.SubmissionMapper, err.Error())
		return
	}

	log.Info("作业批改完成: %s", submission.ID.Hex())
}

// processTimeoutSubmissions 处理超时任务
func (s *HomeworkService) processTimeoutSubmissions(ctx context.Context) {
	timeoutTime := time.Now().Add(-20 * time.Minute)
	submissions, err := s.SubmissionMapper.FindTimeoutSubmissions(ctx, consts.StatusGrading, timeoutTime)
	if err != nil {
		return
	}

	for _, submission := range submissions {
		submission.Status = consts.StatusInitialized
		submission.UpdateTime = time.Now()
		s.SubmissionMapper.Update(ctx, submission)
		log.Info("重置超时任务: %s", submission.ID.Hex())
	}
}

func markSubmissionFailed(ctx context.Context, submission *homework.HomeworkSubmission, submissionMapper *homework.SubmissionMongoMapper, reason string) {
	submission.Status = consts.StatusFailed
	submission.Message = reason
	submission.UpdateTime = time.Now()

	if err := submissionMapper.Update(ctx, submission); err != nil {
		log.Error("标记作业失败状态失败: %v", err)
	} else {
		log.Info("标记作业失败: %s, 原因: %s", submission.ID.Hex(), reason)
	}
}
