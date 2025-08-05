package service

import (
	"context"
	"essay-show/biz/adaptor"
	"essay-show/biz/application/dto/essay/show"
	"essay-show/biz/infrastructure/config"
	"essay-show/biz/infrastructure/consts"
	"essay-show/biz/infrastructure/repository/class"
	"essay-show/biz/infrastructure/repository/homework"
	"essay-show/biz/infrastructure/repository/user"
	"essay-show/biz/infrastructure/util/log"
	"fmt"
	"time"

	"github.com/google/wire"
)

type IHomeworkService interface {
	CreateHomework(ctx context.Context, req *show.CreateHomeworkReq) (*show.CreateHomeworkResp, error)
	ListHomeworks(ctx context.Context, req *show.ListHomeworksReq) (*show.ListHomeworksResp, error)
	GetHomework(ctx context.Context, req *show.GetHomeworkReq) (*show.GetHomeworkResp, error)
	SubmitHomework(ctx context.Context, req *show.SubmitHomeworkReq) (*show.SubmitHomeworkResp, error)
	GetHomeworkSubmissions(ctx context.Context, req *show.GetHomeworkSubmissionsReq) (*show.GetHomeworkSubmissionsResp, error)
	GradeHomework(ctx context.Context, req *show.GradeHomeworkReq) (*show.GradeHomeworkResp, error)
}

type HomeworkService struct {
	HomeworkMapper   *homework.MongoMapper
	SubmissionMapper *homework.SubmissionMongoMapper
	ClassMapper      *class.MongoMapper
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

	// 获取用户详情
	user, err := s.UserMapper.FindOne(ctx, userMeta.GetUserId())
	if err != nil {
		log.Error("获取用户信息失败: %v", err)
		return nil, consts.ErrNotFound
	}

	// 验证班级是否存在
	for _, classID := range req.ClassIds {
		_, err := s.ClassMapper.FindOne(ctx, classID)
		if err != nil {
			log.Error("班级不存在: %v", err)
			return nil, consts.ErrNotFound
		}
	}

	// 创建作业
	now := time.Now()
	deadline := time.Unix(req.Deadline, 0)
	h := &homework.Homework{
		Title:       req.Title,
		Description: req.Description,
		ClassIDs:    req.ClassIds,
		Deadline:    deadline,
		EssayType:   req.EssayType,
		CreatorID:   userMeta.GetUserId(),
		CreatorName: user.Username,
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
		ShareUrl:   fmt.Sprintf("%s?homeworkId=%s", config.GetConfig().Api.ShareHomeWorkURL, h.ID.Hex()),
	}, nil
}

// ListHomeworks 获取作业列表
func (s *HomeworkService) ListHomeworks(ctx context.Context, req *show.ListHomeworksReq) (*show.ListHomeworksResp, error) {
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

	var homeworks []*homework.Homework
	var total int64
	var err error

	homeworks, total, err = s.HomeworkMapper.FindByClassID(ctx, *req.ClassId, page, pageSize)
	if err != nil {
		log.Error("获取作业列表失败: %v", err)
		return nil, consts.ErrGetHomeworkList
	}

	// 转换为响应格式
	homeworkInfos := make([]*show.HomeworkInfo, 0, len(homeworks))
	for _, h := range homeworks {
		// 获取班级名称
		classNames := make([]string, 0, len(h.ClassIDs))
		for _, classID := range h.ClassIDs {
			c, err := s.ClassMapper.FindOne(ctx, classID)
			if err == nil {
				classNames = append(classNames, c.Name)
			}
		}

		homeworkInfos = append(homeworkInfos, &show.HomeworkInfo{
			Id:              h.ID.Hex(),
			Title:           h.Title,
			Description:     h.Description,
			ClassNames:      classNames,
			Deadline:        h.Deadline.Unix(),
			SubmissionCount: 0, // TODO: 获取提交数量
			CreateTime:      h.CreateTime.Unix(),
			CreatorId:       h.CreatorID,
			CreatorName:     h.CreatorName,
		})
	}

	return &show.ListHomeworksResp{
		Homeworks: homeworkInfos,
		Total:     total,
	}, nil
}

// GetHomework 获取作业详情
func (s *HomeworkService) GetHomework(ctx context.Context, req *show.GetHomeworkReq) (*show.GetHomeworkResp, error) {
	// 获取作业信息
	h, err := s.HomeworkMapper.FindOne(ctx, req.HomeworkId)
	if err != nil {
		log.Error("获取作业信息失败: %v", err)
		return nil, consts.ErrNotFound
	}

	// 获取班级名称
	classNames := make([]string, 0, len(h.ClassIDs))
	for _, classID := range h.ClassIDs {
		c, err := s.ClassMapper.FindOne(ctx, classID)
		if err == nil {
			classNames = append(classNames, c.Name)
		}
	}

	homeworkDetail := &show.HomeworkDetail{
		Id:              h.ID.Hex(),
		Title:           h.Title,
		Description:     h.Description,
		ClassIds:        h.ClassIDs,
		ClassNames:      classNames,
		Deadline:        h.Deadline.Unix(),
		EssayType:       h.EssayType,
		SubmissionCount: 0, // TODO: 获取提交数量
		CreateTime:      h.CreateTime.Unix(),
		CreatorId:       h.CreatorID,
		CreatorName:     h.CreatorName,
	}

	return &show.GetHomeworkResp{
		Homework: homeworkDetail,
	}, nil
}

// SubmitHomework 提交作业
func (s *HomeworkService) SubmitHomework(ctx context.Context, req *show.SubmitHomeworkReq) (*show.SubmitHomeworkResp, error) {
	// 验证作业是否存在
	//h, err := s.HomeworkMapper.FindOne(ctx, req.HomeworkId)
	// if err != nil {
	// 	log.Error("作业不存在: %v", err)
	// 	return nil, consts.ErrNotFound
	// }

	// resultChan := make(chan string, 100)

	// 启动批改服务
	// go func(ctx context.Context) {
	// 	p := provider.Get()
	// 	defer close(resultChan)
	// 	p.EssayService.EssayEvaluateStream(ctx, &req, resultChan)
	// }(ctx)

	// // 实时转发流式数据
	// for jsonMessage := range resultChan {
	// 	var msgData util.StreamMessage
	// 	json.Unmarshal([]byte(jsonMessage), &msgData)
	// 	if msgData.Type == util.STComplete {
	// 		break
	// 	}
	// 	if msgData.Type == util.STError {
	// 		log.CtxInfo(ctx, "resp=%+v", msgData)
	// 		break
	// 	}
	// }

	// 自动批改作文
	// gradeResult := ""
	// if req.Text != "" {
	// 	evaluateReq := &show.EssayEvaluateReq{
	// 		Title: req.Title,
	// 		Text:  req.Text,
	// 	}

	// 	resp, err := s.EssayService.EssayEvaluateStream(ctx, evaluateReq)
	// 	if err == nil {
	// 		gradeResult = resp.Response
	// 	}
	// }

	// // 创建作业提交记录
	// now := time.Now()
	// submission := &homework.HomeworkSubmission{
	// 	HomeworkID:  req.HomeworkId,
	// 	StudentName: req.StudentName,
	// 	Title:       req.Title,
	// 	Text:        req.Text,
	// 	Images:      req.Images,
	// 	GradeResult: gradeResult,
	// 	IsGraded:    false,
	// 	SubmitTime:  now,
	// 	CreateTime:  now,
	// 	UpdateTime:  now,
	// }

	// err = s.SubmissionMapper.Insert(ctx, submission)
	// if err != nil {
	// 	log.Error("提交作业失败: %v", err)
	// 	return nil, consts.ErrSubmitHomework
	// }

	// return &show.SubmitHomeworkResp{
	// 	SubmissionId: submission.ID.Hex(),
	// 	GradeResult:  gradeResult,
	// }, nil
	return nil, nil
}

// GetHomeworkSubmissions 获取作业提交列表
func (s *HomeworkService) GetHomeworkSubmissions(ctx context.Context, req *show.GetHomeworkSubmissionsReq) (*show.GetHomeworkSubmissionsResp, error) {
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

	// 获取作业提交记录
	submissions, total, err := s.SubmissionMapper.FindByHomeworkID(ctx, req.HomeworkId, page, pageSize)
	if err != nil {
		log.Error("获取作业提交列表失败: %v", err)
		return nil, consts.ErrGetHomeworkList
	}

	// 转换为响应格式
	submissionInfos := make([]*show.SubmissionInfo, 0, len(submissions))
	for _, s := range submissions {
		submissionInfos = append(submissionInfos, &show.SubmissionInfo{
			Id:          s.ID.Hex(),
			StudentName: s.StudentName,
			Title:       s.Title,
			Text:        s.Text,
			Images:      s.Images,
			GradeResult: s.GradeResult,
			SubmitTime:  s.SubmitTime.Unix(),
			IsGraded:    s.IsGraded,
		})
	}

	return &show.GetHomeworkSubmissionsResp{
		Submissions: submissionInfos,
		Total:       total,
	}, nil
}

// GradeHomework 批改作业
func (s *HomeworkService) GradeHomework(ctx context.Context, req *show.GradeHomeworkReq) (*show.GradeHomeworkResp, error) {
	// 获取用户信息
	userMeta := adaptor.ExtractUserMeta(ctx)
	if userMeta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}

	// 获取提交记录
	submission, err := s.SubmissionMapper.FindOne(ctx, req.SubmissionId)
	if err != nil {
		log.Error("获取提交记录失败: %v", err)
		return nil, consts.ErrNotFound
	}

	// 更新批改结果
	submission.GradeResult = req.GradeResult
	submission.Comment = req.Comment
	submission.IsGraded = true
	submission.UpdateTime = time.Now()

	err = s.SubmissionMapper.Update(ctx, submission)
	if err != nil {
		log.Error("更新批改结果失败: %v", err)
		return nil, consts.ErrGradeHomework
	}

	return &show.GradeHomeworkResp{
		Code: 0,
		Msg:  "批改成功",
	}, nil
}
