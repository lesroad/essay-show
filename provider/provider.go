package provider

import (
	"essay-show/biz/application/service"
	"essay-show/biz/infrastructure/cache"
	"essay-show/biz/infrastructure/config"
	"essay-show/biz/infrastructure/repository/attend"
	"essay-show/biz/infrastructure/repository/class"
	"essay-show/biz/infrastructure/repository/exercise"
	"essay-show/biz/infrastructure/repository/feedback"
	"essay-show/biz/infrastructure/repository/homework"
	"essay-show/biz/infrastructure/repository/invitation"
	"essay-show/biz/infrastructure/repository/log"
	"essay-show/biz/infrastructure/repository/question_bank"
	"essay-show/biz/infrastructure/repository/user"

	"github.com/google/wire"
)

var provider *Provider

func Init() {
	var err error
	provider, err = NewProvider()
	if err != nil {
		panic(err)
	}
}

// Provider 提供controller依赖的对象
type Provider struct {
	Config              *config.Config
	UserService         service.UserService
	EssayService        service.EssayService
	StsService          service.StsService
	ExerciseService     service.ExerciseService
	FeedBackService     service.FeedBackService
	ClassService        service.IClassService
	HomeworkService     service.IHomeworkService
	QuestionBankService service.IQuestionBankService
}

func Get() *Provider {
	return provider
}

// var RpcSet = wire.NewSet(
// 	platform_sts.PlatformStsSet,
// )

var ApplicationSet = wire.NewSet(
	service.UserServiceSet,
	service.EssayServiceSet,
	service.StsServiceSet,
	service.ExerciseServiceSet,
	service.FeedbackServiceSet,
	service.ClassServiceSet,
	service.HomeworkServiceSet,
	service.QuestionBankServiceSet,
)

var InfrastructureSet = wire.NewSet(
	// Configuration
	config.NewConfig,

	// Repository Layer (Data Persistence)
	user.NewMongoMapper,
	log.NewMongoMapper,
	exercise.NewMongoMapper,
	attend.NewMongoMapper,
	invitation.NewCodeMongoMapper,
	invitation.NewLogMongoMapper,
	feedback.NewMongoMapper,
	class.NewMongoMapper,
	class.NewMemberMongoMapper,
	homework.NewMongoMapper,
	homework.NewSubmissionMongoMapper,
	question_bank.NewMySQLMapperFromConfig,

	// Cache Layer
	cache.NewDownloadCacheMapper,

	//RpcSet,
)

var AllProvider = wire.NewSet(
	ApplicationSet,
	InfrastructureSet,
)
