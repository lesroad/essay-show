package service

import (
	"context"
	"essay-show/biz/application/dto/essay/show"
	"essay-show/biz/infrastructure/repository/question_bank"
	"essay-show/biz/infrastructure/util/log"

	"github.com/google/wire"
)

type IQuestionBankService interface {
	ListQuestionBanks(ctx context.Context, req *show.ListQuestionBanksReq) (*show.ListQuestionBanksResp, error)
}

type QuestionBankService struct {
	QuestionBankMapper *question_bank.MySQLMapper
}

var QuestionBankServiceSet = wire.NewSet(
	wire.Struct(new(QuestionBankService), "*"),
	wire.Bind(new(IQuestionBankService), new(*QuestionBankService)),
)

// ListQuestionBanks 获取题库列表
func (s *QuestionBankService) ListQuestionBanks(ctx context.Context, req *show.ListQuestionBanksReq) (*show.ListQuestionBanksResp, error) {

	// 调用数据层获取题库列表
	questionBanks, total, err := s.QuestionBankMapper.ListQuestionBanks(ctx, req)
	if err != nil {
		log.Error("Failed to get question banks from database: %v", err)
		return nil, err
	}

	log.Info("Successfully retrieved %d question banks, total: %d", len(questionBanks), total)

	return &show.ListQuestionBanksResp{
		QuestionBanks: questionBanks,
		Total:         total,
	}, nil
}
