package service

import (
	"context"
	"encoding/json"
	"essay-show/biz/adaptor"
	"essay-show/biz/application/dto/essay/show"
	"essay-show/biz/application/dto/essay/stateless"
	"essay-show/biz/infrastructure/config"
	"essay-show/biz/infrastructure/consts"
	"essay-show/biz/infrastructure/repository/class"
	"essay-show/biz/infrastructure/repository/homework"
	"essay-show/biz/infrastructure/repository/user"
	"essay-show/biz/infrastructure/util"
	"essay-show/biz/infrastructure/util/log"
	"fmt"
	"time"

	"github.com/google/wire"
	"github.com/spf13/cast"
)

type IHomeworkService interface {
	CreateHomework(ctx context.Context, req *show.CreateHomeworkReq) (*show.CreateHomeworkResp, error)
	ListHomeworks(ctx context.Context, req *show.ListHomeworksReq) (*show.ListHomeworksResp, error)
	ListHomeworksForStu(ctx context.Context, req *show.ListHomeworksForStuReq) (*show.ListHomeworksForStuResp, error)
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
	c, err := s.ClassMapper.FindOne(ctx, req.ClassId)
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
		ShareUrl:   fmt.Sprintf("%s/homework/list/student?classId=%s", config.GetConfig().Api.MiniProgramURL, c.ID.Hex()),
	}, nil
}

// ListHomeworks 获取作业列表(教师端)
func (s *HomeworkService) ListHomeworks(ctx context.Context, req *show.ListHomeworksReq) (*show.ListHomeworksResp, error) {
	// 获取用户信息
	userMeta := adaptor.ExtractUserMeta(ctx)
	if userMeta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
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

	// 获取班级信息
	c, err := s.ClassMapper.FindOne(ctx, req.ClassId)
	if err != nil {
		log.Error("班级不存在: %v", err)
		return nil, consts.ErrNotFound
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

	homeworks, total, err := s.HomeworkMapper.FindByClassID(ctx, req.ClassId, page, pageSize)
	if err != nil {
		log.Error("获取作业列表失败: %v", err)
		return nil, consts.ErrGetHomeworkList
	}

	// 转换为响应格式
	homeworkInfos := make([]*show.HomeworkInfo, 0, len(homeworks))
	for _, h := range homeworks {
		// 获取提交数量
		_, submitCount, err := s.SubmissionMapper.FindByHomeworkID(ctx, h.ID.Hex(), 1, 5)
		if err != nil {
			log.Error("获取提交情况失败: %v", err)
			return nil, consts.ErrGetHomeworkList
		}

		// 获取已批改数量
		gradeList, err := s.SubmissionMapper.FindByStatus(ctx, consts.StatusCompleted)
		if err != nil {
			log.Error("获取已批改数量失败: %v", err)
			return nil, consts.ErrGetHomeworkList
		}

		homeworkInfos = append(homeworkInfos, &show.HomeworkInfo{
			Id:              h.ID.Hex(),
			Title:           h.Title,
			Description:     h.Description,
			CreateTime:      h.CreateTime.Unix(),
			SubmissionCount: submitCount,
			StudentCount:    c.MemberCount - 1,
			GradeCount:      int64(len(gradeList)),
		})
	}

	return &show.ListHomeworksResp{
		Homeworks: homeworkInfos,
		Total:     total,
	}, nil
}

// ListHomeworksForStu 获取作业列表(学生端)
func (s *HomeworkService) ListHomeworksForStu(ctx context.Context, req *show.ListHomeworksForStuReq) (*show.ListHomeworksForStuResp, error) {
	// 获取用户信息
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

	// 检查是否已经是班级成员
	_, err = s.MemberMapper.FindByClassIDAndStuID(ctx, req.ClassId, userMeta.GetUserId())
	if err == consts.ErrNotFound {
		log.Error("用户不是班级成员")
		return nil, consts.ErrNotClassMember
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

	homeworks, total, err := s.HomeworkMapper.FindByClassID(ctx, req.ClassId, page, pageSize)
	if err != nil {
		log.Error("获取作业列表失败: %v", err)
		return nil, consts.ErrGetHomeworkList
	}

	// 转换为响应格式
	submissionInfos := make([]*show.SubmissionInfo, 0, len(homeworks))
	for _, h := range homeworks {
		// 获取提交状态
		submission, err := s.SubmissionMapper.FindByStudentAndHomework(ctx, userMeta.GetUserId(), h.ID.Hex())
		switch {
		case err == consts.ErrNotFound:
			submission = &homework.HomeworkSubmission{
				Status: consts.StatusNotSubmission,
			}
		case err != nil:
			log.Error("获取提交情况失败: %v", err)
			return nil, consts.ErrGetHomeworkList
		default:
		}

		submissionInfos = append(submissionInfos, &show.SubmissionInfo{
			Id:          h.ID.Hex(),
			StudentName: u.Username,
			Title:       h.Title,
			Description: h.Description,
			CreateTime:  h.CreateTime.Unix(),
			Status:      int64(submission.Status),
			GradeResult: submission.GradeResult,
			SubmitTime:  submission.CreateTime.Unix(),
		})
	}

	return &show.ListHomeworksForStuResp{
		Homeworks: submissionInfos,
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
			continue
		}

		// 获取学生信息
		user, err := s.UserMapper.FindOne(ctx, m.UserID)
		if err != nil {
			log.Error("获取学生信息失败: %v", err)
			return nil, consts.ErrGetSubmission
		}

		sub := &show.SubmissionInfo{
			StudentName: user.Username,
			Title:       h.Title,
			Description: h.Description,
		}

		// 查询学生提交记录
		userSubmission, err := s.SubmissionMapper.FindByStudentAndHomework(ctx, user.ID.Hex(), req.HomeworkId)
		switch {
		case err == consts.ErrNotFound:
			sub.Status = consts.StatusNotSubmission
		case err != nil:
			log.Error("获取学生提交记录失败: %v", err)
			return nil, consts.ErrGetSubmission
		default:
			sub.Id = userSubmission.ID.Hex()
			sub.GradeResult = userSubmission.GradeResult
			sub.SubmitTime = userSubmission.CreateTime.Unix()
			sub.Status = int64(userSubmission.Status)
			sub.CreateTime = userSubmission.CreateTime.Unix()
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

	// 更新为批改中
	submission.Status = consts.StatusGrading
	submission.UpdateTime = time.Now()
	s.SubmissionMapper.Update(ctx, submission)

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

	resultChan := make(chan string, 100)
	var finalResult string

	// 调用批改服务
	go func() {
		defer close(resultChan)
		client.EvaluateStream(ctx, title, text, &grade, &essayType, &prompt, resultChan)
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
	submission.GradeResult = cast.ToString(evaluateResult.AIEvaluation.ScoreEvaluation.Scores.All)
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
