package service

import (
	"context"
	"essay-show/biz/adaptor"
	"essay-show/biz/application/dto/essay/show"
	"essay-show/biz/infrastructure/consts"
	"essay-show/biz/infrastructure/repository/homework"
	"essay-show/biz/infrastructure/repository/user"
	"essay-show/biz/infrastructure/util/log"

	"github.com/google/wire"
)

type IAdminService interface {
	GetAdminHomeworkStatistics(ctx context.Context, req *show.GetAdminHomeworkStatisticsReq) (*show.GetAdminHomeworkStatisticsResp, error)
}

type AdminService struct {
	HomeworkMapper   *homework.MongoMapper
	UserMapper       *user.MongoMapper
	SubmissionMapper *homework.SubmissionMongoMapper
}

var AdminServiceSet = wire.NewSet(
	wire.Struct(new(AdminService), "*"),
	wire.Bind(new(IAdminService), new(*AdminService)),
)

func (s *AdminService) GetAdminHomeworkStatistics(ctx context.Context, req *show.GetAdminHomeworkStatisticsReq) (*show.GetAdminHomeworkStatisticsResp, error) {
	userMeta := adaptor.ExtractUserMeta(ctx)
	if userMeta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}

	user, err := s.UserMapper.FindOne(ctx, userMeta.GetUserId())
	if err != nil {
		log.Error("获取用户信息失败: %v", err)
		return nil, consts.ErrNotFound
	}

	if user.Role != consts.RoleAdmin {
		return nil, consts.ErrNotAuthentication
	}

	var (
		page     int64 = int64(1)
		pageSize int64 = int64(10)
	)
	if req.PaginationOptions != nil {
		if req.PaginationOptions.Page != nil {
			page = *req.PaginationOptions.Page
		}
		if req.PaginationOptions.Limit != nil {
			pageSize = *req.PaginationOptions.Limit
		}
	}

	homeworks, total, err := s.HomeworkMapper.FindHomeworks(ctx, page, pageSize, req.Topic, req.StartTime, req.EndTime)
	if err != nil {
		log.Error("获取作业列表失败: %v", err)
		return nil, consts.ErrNotFound
	}

	statistics := make([]*show.HomeworkStatistics, 0, len(homeworks))
	for _, homework := range homeworks {
		homeworkStatistics := &show.HomeworkStatistics{
			Id:          homework.ID.Hex(),
			Title:       homework.Title,
			Description: homework.Description,
			Standard:    *homework.Standard,
			Submissions: make([]*show.SubmissionStatistics, 0),
		}

		submissions, err := s.SubmissionMapper.FindAllByHomework(ctx, homework.ID.Hex(), &[]int{consts.StatusCompleted, consts.StatusModified})
		if err != nil {
			log.Error("获取作业提交列表失败: %v", err)
			return nil, consts.ErrNotFound
		}
		for _, submission := range submissions {
			homeworkStatistics.Submissions = append(homeworkStatistics.Submissions, &show.SubmissionStatistics{
				Id:               submission.ID.Hex(),
				MemberId:         submission.MemberId,
				Title:            submission.Title,
				Text:             submission.Text,
				Images:           submission.Images,
				CorrectionResult: submission.Response,
			})
		}
		statistics = append(statistics, homeworkStatistics)
	}

	return &show.GetAdminHomeworkStatisticsResp{Statistics: statistics, Total: total}, nil
}
