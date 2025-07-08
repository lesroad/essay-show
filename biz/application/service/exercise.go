package service

import (
	"errors"
	"essay-show/biz/adaptor"
	"essay-show/biz/application/dto/essay/show"
	"essay-show/biz/infrastructure/consts"
	"essay-show/biz/infrastructure/repository/exercise"
	"essay-show/biz/infrastructure/repository/log"
	"essay-show/biz/infrastructure/repository/user"
	"essay-show/biz/infrastructure/util"
	eu "essay-show/biz/infrastructure/util/exercise"
	logx "essay-show/biz/infrastructure/util/log"
	"time"

	"github.com/google/wire"
	"github.com/jinzhu/copier"
	"golang.org/x/net/context"
)

type IExerciseService interface {
	CreateExercise(ctx context.Context, req *show.CreateExerciseReq) (resp *show.CreateExerciseResp, err error)
	CreateExerciseStream(ctx context.Context, req *show.CreateExerciseReq, resultChan chan<- string) error
	ListSimpleExercises(ctx context.Context, req *show.ListSimpleExercisesReq) (resp *show.ListSimpleExercisesResp, err error)
	GetExercise(ctx context.Context, req *show.GetExerciseReq) (resp *show.GetExerciseResp, err error)
	DoExercise(ctx context.Context, req *show.DoExerciseReq) (resp *show.DoExerciseResp, err error)
	LikeExercise(ctx context.Context, req *show.LikeExerciseReq) (resp *show.Response, err error)
}

type ExerciseService struct {
	ExerciseMapper *exercise.MongoMapper
	LogMapper      *log.MongoMapper
	UserMapper     *user.MongoMapper
}

var ExerciseServiceSet = wire.NewSet(
	wire.Struct(new(ExerciseService), "*"),
	wire.Bind(new(IExerciseService), new(*ExerciseService)),
)

// CreateExercise 创建一套练习
func (s ExerciseService) CreateExercise(ctx context.Context, req *show.CreateExerciseReq) (resp *show.CreateExerciseResp, err error) {
	// 获取批改记录
	l, err := s.LogMapper.FindOne(ctx, req.LogId)
	if err != nil {
		logx.Error("获取批改记录失败, err:%v", err.Error())
		return nil, consts.ErrInvalidObjectId
	}

	// 获取用户信息
	userMeta := adaptor.ExtractUserMeta(ctx)
	if userMeta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}
	u, err := s.UserMapper.FindOne(ctx, userMeta.UserId)
	if err != nil {
		logx.Error("获取用户信息失败, err:%v", err.Error())
		return nil, consts.ErrNotAuthentication
	}

	// 调用生成接口
	e, err := eu.GenerateExercise(ctx, u.Grade, l)
	if err != nil {
		logx.Error("生成练习失败, err:%v", err.Error())
		return nil, consts.ErrCreateExercise
	}

	// 存储练习
	e.LogId = req.LogId
	e.UserId = userMeta.UserId
	err = s.ExerciseMapper.Insert(ctx, e)
	if err != nil {
		logx.Error("存储练习失败, err:%v", err.Error())
		return nil, consts.ErrCreateExercise
	}

	// dto构造
	dto := &show.Exercise{}
	err = copier.Copy(dto, e)
	if err != nil {
		return nil, err
	}
	dto.Id = e.ID.Hex()
	dto.CreateTime = e.CreateTime.Unix()
	dto.UpdateTime = e.CreateTime.Unix()

	resp = &show.CreateExerciseResp{
		Code:     0,
		Msg:      "success",
		Exercise: dto,
	}
	return
}

// ListSimpleExercises 获取简要的练习列表
func (s ExerciseService) ListSimpleExercises(ctx context.Context, req *show.ListSimpleExercisesReq) (*show.ListSimpleExercisesResp, error) {
	// 获取用户信息
	userMeta := adaptor.ExtractUserMeta(ctx)
	if userMeta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}

	// 查询批改记录对应的练习
	data, total, err := s.ExerciseMapper.FindManyByLogId(ctx, req.LogId, req.PaginationOptions)
	if err != nil && !errors.Is(err, consts.ErrNotFound) {
		return nil, consts.ErrNotFound
	}

	// 构造dto切片
	dtos := make([]*show.ListSimpleExercisesResp_SimpleExercise, 0)
	for _, v := range data {
		records := make([]*show.ListSimpleExercisesResp_Record, 0)
		dto := &show.ListSimpleExercisesResp_SimpleExercise{
			Id:         v.ID.Hex(),
			TotalScore: -1,
			Records:    records,
			FinishTime: time.Time{}.Unix(),
			Like:       v.Like,
		}
		// 该练习有过提交记录
		if len(v.History.Records) > 0 {
			// 获取最后一次提交记录
			lastRecord := v.History.Records[len(v.History.Records)-1]
			// 获取最后一次记录各题的得分
			for _, r := range lastRecord.Records {
				records = append(records, &show.ListSimpleExercisesResp_Record{
					Id:    r.Id,
					Score: r.Score,
				})
			}
			// 记录最后一次作答情况
			dto.TotalScore = lastRecord.Score
			dto.FinishTime = lastRecord.CreateTime.Unix()
		} else {
			// 无作答记录则均用-1占位
			for _, cq := range v.Question.ChoiceQuestions {
				records = append(records, &show.ListSimpleExercisesResp_Record{
					Id:    cq.Id,
					Score: -1,
				})
			}
		}
		dto.Records = records
		dtos = append(dtos, dto)
	}

	return &show.ListSimpleExercisesResp{
		Code:      0,
		Msg:       "success",
		Exercises: dtos,
		Total:     total,
	}, nil

}

// GetExercise 获取一次练习的详细记录
func (s ExerciseService) GetExercise(ctx context.Context, req *show.GetExerciseReq) (*show.GetExerciseResp, error) {
	// 查询练习
	e, err := s.ExerciseMapper.FindOneById(ctx, req.Id)
	if err != nil {
		return nil, consts.ErrNotFound
	}
	// 处理选择题切片
	cqs := make([]*show.ChoiceQuestion, 0)
	for _, v := range e.Question.ChoiceQuestions {
		// 处理各个选项
		ops := make([]*show.Option, 0)
		for _, o := range v.Options {
			ops = append(ops, &show.Option{
				Option:  o.Option,
				Content: o.Content,
				Score:   o.Score,
			})
		}
		cq := &show.ChoiceQuestion{
			Id:          v.Id,
			Question:    v.Question,
			Explanation: v.Explanation,
			Options:     ops,
		}
		cqs = append(cqs, cq)
	}

	// 处理答题记录
	rds := make([]*show.Records, 0)
	for _, v := range e.History.Records {
		rs := make([]*show.Record, 0)
		for _, r := range v.Records {
			rs = append(rs, &show.Record{
				Id:     r.Id,
				Option: r.Option,
				Score:  r.Score,
			})
		}
		rds = append(rds, &show.Records{
			Records:    rs,
			Score:      v.Score,
			CreateTime: v.CreateTime.Unix(),
		})
	}
	// 构造dto
	dto := &show.Exercise{
		Id:         e.ID.Hex(),
		UserId:     e.UserId,
		LogId:      e.LogId,
		Question:   &show.Question{ChoiceQuestions: cqs},
		History:    &show.History{Records: rds},
		Like:       e.Like,
		CreateTime: e.CreateTime.Unix(),
		UpdateTime: e.UpdateTime.Unix(),
		Status:     e.Status,
	}

	return &show.GetExerciseResp{
		Code:     0,
		Msg:      "success",
		Exercise: dto,
	}, nil
}

// DoExercise 提交一次练习作答，目前是没有暂时记录的，需要完成所有的题目然后结算
func (s ExerciseService) DoExercise(ctx context.Context, req *show.DoExerciseReq) (resp *show.DoExerciseResp, err error) {
	e, err := s.ExerciseMapper.FindOneById(ctx, req.Id)
	if err != nil {
		return nil, consts.ErrNotFound
	}

	// 初始化
	if e.History == nil {
		rds := make([]*exercise.Records, 0)
		e.History = &exercise.History{Records: rds}
	}

	// 用map存储题目id与题目
	cqs := e.Question.ChoiceQuestions
	qMap := make(map[string]*exercise.ChoiceQuestion)
	for _, v := range cqs {
		qMap[v.Id] = v
	}

	// 做题记录
	rs := make([]*exercise.Record, 0)
	var sum int64
	for _, v := range req.Records {
		// 根据id获取题目
		if q, ok := qMap[v.Id]; ok {
			var score int64
			for _, o := range q.Options {
				if o.Option == v.Option {
					score = o.Score
				}
			}
			// 构造单题记录
			r := &exercise.Record{
				Id:     q.Id,
				Option: v.Option,
				Score:  score,
			}
			sum += score
			rs = append(rs, r)
		}
	}
	// 构造练习作答记录
	rds := &exercise.Records{
		Records:    rs,
		Score:      sum,
		CreateTime: time.Now(),
	}

	// 更新记录
	e.History.Records = append(e.History.Records, rds)
	err = s.ExerciseMapper.Update(ctx, e)
	if err != nil {
		logx.Error("更新练习记录失败")
		return nil, consts.ErrDoExercise
	}

	// 将最新的记录返回
	rsDto := make([]*show.Record, 0)
	for _, v := range e.History.Records[len(e.History.Records)-1].Records {
		rsDto = append(rsDto, &show.Record{
			Id:     v.Id,
			Option: v.Option,
			Score:  v.Score,
		})
	}
	dto := &show.Records{
		Records:    rsDto,
		Score:      rds.Score,
		CreateTime: rds.CreateTime.Unix(),
	}
	resp = &show.DoExerciseResp{
		Code:    0,
		Msg:     "success",
		Records: dto,
	}
	return
}

// LikeExercise 点赞或点踩一个练习
func (s ExerciseService) LikeExercise(ctx context.Context, req *show.LikeExerciseReq) (resp *show.Response, err error) {
	// 查询练习
	e, err := s.ExerciseMapper.FindOneById(ctx, req.Id)
	if err != nil {
		return nil, consts.ErrNotFound
	}
	// 校验参数
	if req.Like > 1 || req.Like < -1 {
		return &show.Response{
			Code: 999,
			Msg:  "Like行为超出范围",
		}, nil
	}
	// 更改点赞状态
	e.Like = req.Like
	err = s.ExerciseMapper.Update(ctx, e)
	if err != nil {
		return util.Fail(999, "标记失败"), nil
	}

	return util.Succeed("标记成功")
}

// CreateExerciseStream 流式创建练习
func (s ExerciseService) CreateExerciseStream(ctx context.Context, req *show.CreateExerciseReq, resultChan chan<- string) error {
	// 获取批改记录
	l, err := s.LogMapper.FindOne(ctx, req.LogId)
	if err != nil {
		logx.Error("获取批改记录失败, err:%v", err.Error())
		util.SendStreamMessage(resultChan, util.STError, "获取批改记录失败", nil)
		return consts.ErrInvalidObjectId
	}

	// 获取用户信息
	userMeta := adaptor.ExtractUserMeta(ctx)
	if userMeta.GetUserId() == "" {
		util.SendStreamMessage(resultChan, util.STError, "用户未认证", nil)
		return consts.ErrNotAuthentication
	}

	u, err := s.UserMapper.FindOne(ctx, userMeta.UserId)
	if err != nil {
		logx.Error("获取用户信息失败, err:%v", err.Error())
		util.SendStreamMessage(resultChan, util.STError, "获取用户信息失败", nil)
		return consts.ErrNotAuthentication
	}

	e, err := eu.GenerateExerciseStream(ctx, u.Grade, l, resultChan)
	if err != nil {
		logx.Error("生成练习失败, err:%v", err.Error())
		util.SendStreamMessage(resultChan, util.STError, "生成练习失败", nil)
		return err
	}

	// 存储练习
	e.LogId = req.LogId
	e.UserId = userMeta.UserId
	err = s.ExerciseMapper.Insert(ctx, e)
	if err != nil {
		logx.Error("存储练习失败, err:%v", err.Error())
		util.SendStreamMessage(resultChan, util.STError, "存储练习失败", nil)
		return consts.ErrCreateExercise
	}

	// dto构造
	dto := &show.Exercise{}
	err = copier.Copy(dto, e)
	if err != nil {
		return err
	}
	dto.Id = e.ID.Hex()
	dto.CreateTime = e.CreateTime.Unix()
	dto.UpdateTime = e.CreateTime.Unix()

	util.SendStreamMessage(resultChan, util.STComplete, "练习生成完成", dto)
	return nil
}
