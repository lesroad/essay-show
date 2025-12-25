package invitation

import (
	"errors"
	"essay-show/biz/infrastructure/config"
	"essay-show/biz/infrastructure/consts"
	"time"

	"github.com/zeromicro/go-zero/core/stores/monc"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/net/context"
)

const (
	logPrefixUserCacheKey = "cache:invitation_log"
	logCollectionName     = "invitation_log"
)

type ILogMongoMapper interface {
	Insert(ctx context.Context, inviter string, invitee string) error
	FindOneByInvitee(ctx context.Context, invitee string) (*Log, error)
}

type LogMongoMapper struct {
	conn *monc.Model
}

func NewLogMongoMapper(config *config.Config) *LogMongoMapper {
	conn := monc.MustNewModel(config.Mongo.URL, config.Mongo.DB, logCollectionName, config.Cache)
	return &LogMongoMapper{
		conn: conn,
	}
}

func (m *LogMongoMapper) Insert(ctx context.Context, inviter string, invitee string, source *string) error {
	l := Log{
		ID:        primitive.NewObjectID(),
		Inviter:   inviter,
		Invitee:   invitee,
		Source:    source,
		Timestamp: time.Now(),
	}
	_, err := m.conn.InsertOneNoCache(ctx, &l)
	return err
}

func (m *LogMongoMapper) FindOneByInvitee(ctx context.Context, invitee string) (*Log, error) {
	l := &Log{}
	err := m.conn.FindOneNoCache(ctx, l, bson.M{"invitee": invitee})
	switch {
	case err == nil:
		return l, nil
	case errors.Is(err, mongo.ErrNoDocuments):
		return nil, consts.ErrNotFound
	default:
		return nil, err
	}
}
