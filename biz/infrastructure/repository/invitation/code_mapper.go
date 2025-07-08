package invitation

import (
	"errors"
	"essay-show/biz/infrastructure/config"
	"essay-show/biz/infrastructure/consts"
	"math/rand"
	"time"

	"github.com/zeromicro/go-zero/core/stores/monc"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/net/context"
)

const (
	codePrefixUserCacheKey = "cache:invitation_code"
	codeCollectionName     = "invitation_code"
	letters                = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digits                 = "0123456789"
)

type ICodeMongoMapper interface {
	Insert(ctx context.Context, userId string) error
	FindOneByUserId(ctx context.Context, userId string) (*Code, error)
}

type CodeMongoMapper struct {
	conn *monc.Model
}

func NewCodeMongoMapper(config *config.Config) *CodeMongoMapper {
	conn := monc.MustNewModel(config.Mongo.URL, config.Mongo.DB, codeCollectionName, config.Cache)
	return &CodeMongoMapper{
		conn: conn,
	}
}

func (m *CodeMongoMapper) Insert(ctx context.Context, userId string) (*Code, error) {
	_code := genCode()
	t := 0
	for t < 100 {
		c, err := m.FindOneByCode(ctx, _code)
		if err == nil && c != nil {
			t++
			continue
		}
		break
	}

	c := &Code{
		ID:        primitive.NewObjectID(),
		UserId:    userId,
		Code:      _code,
		Timestamp: time.Now(),
	}

	_, err := m.conn.InsertOneNoCache(ctx, c)
	return c, err
}

func (m *CodeMongoMapper) FindOneByUserId(ctx context.Context, userId string) (*Code, error) {
	c := &Code{}
	err := m.conn.FindOneNoCache(ctx, c, bson.M{consts.UserID: userId})
	switch {
	case err == nil:
		return c, nil
	case errors.Is(err, mongo.ErrNoDocuments):
		return nil, consts.ErrNotFound
	default:
		return nil, err
	}
}

func (m *CodeMongoMapper) FindOneByCode(ctx context.Context, code string) (*Code, error) {
	c := &Code{}
	err := m.conn.FindOneNoCache(ctx, c, bson.M{"code": code})
	switch {
	case err == nil:
		return c, nil
	case errors.Is(err, mongo.ErrNoDocuments):
		return nil, consts.ErrNotFound
	default:
		return nil, err
	}
}

func genCode() string {
	// 生成四位大写字母
	letterPart := make([]byte, 4)
	for i := range letterPart {
		letterPart[i] = letters[rand.Intn(len(letters))]
	}

	// 生成两位数字
	digitPart := make([]byte, 2)
	for i := range digitPart {
		digitPart[i] = digits[rand.Intn(len(digits))]
	}

	// 将字母和数字混合
	code := append(letterPart, digitPart...)
	rand.Shuffle(len(code), func(i, j int) {
		code[i], code[j] = code[j], code[i]
	})

	return string(code)
}
