package class

import (
	"context"
	"errors"
	"essay-show/biz/infrastructure/config"
	"essay-show/biz/infrastructure/consts"
	"essay-show/biz/infrastructure/util/log"
	"time"

	"github.com/zeromicro/go-zero/core/stores/monc"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ClassMember struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ClassID    string             `bson:"class_id" json:"classId"`
	Name       string             `bson:"name" json:"name"`
	UserID     *string            `bson:"user_id" json:"userId"`
	JoinTime   *time.Time         `bson:"join_time" json:"joinTime"`
	CreateTime time.Time          `bson:"create_time" json:"createTime"`
	UpdateTime time.Time          `bson:"update_time" json:"updateTime"`
}

const (
	prefixMemberCacheKey = "cache:class_member"
	MemberCollectionName = "class_member"
)

type MemberMongoMapper struct {
	conn *monc.Model
}

func NewMemberMongoMapper(config *config.Config) *MemberMongoMapper {
	log.Info("NewMemberMongoMapper config: %v, collection: %s", config, MemberCollectionName)
	conn := monc.MustNewModel(config.Mongo.URL, config.Mongo.DB, MemberCollectionName, config.Cache)
	return &MemberMongoMapper{
		conn: conn,
	}
}

func (m *MemberMongoMapper) Insert(ctx context.Context, member *ClassMember) error {
	if member.ID.IsZero() {
		member.ID = primitive.NewObjectID()
		member.CreateTime = time.Now()
		member.UpdateTime = time.Now()
	}
	_, err := m.conn.InsertOneNoCache(ctx, member)
	return err
}

func (m *MemberMongoMapper) FindByClassID(ctx context.Context, classID string, page, pageSize int64) ([]*ClassMember, int64, error) {
	var members []*ClassMember
	filter := bson.M{"class_id": classID}

	// 获取总数
	total, err := m.conn.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	// 分页查询
	skip := (page - 1) * pageSize
	err = m.conn.Find(ctx, &members, filter, &options.FindOptions{
		Skip:  &skip,
		Limit: &pageSize,
		Sort:  bson.M{"name": 1}, // 按名字正序
	})
	if err != nil {
		return nil, 0, err
	}

	return members, total, nil
}

func (m *MemberMongoMapper) FindByStuID(ctx context.Context, userID string) ([]*ClassMember, int64, error) {
	var members []*ClassMember
	filter := bson.M{"user_id": userID, "role": consts.RoleStudent}

	total, err := m.conn.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	err = m.conn.Find(ctx, &members, filter, &options.FindOptions{
		Sort: bson.M{"join_time": -1},
	})
	if err != nil {
		return nil, 0, err
	}

	return members, total, nil
}

func (m *MemberMongoMapper) FindByClassIDAndStuID(ctx context.Context, classID, userID string) (*ClassMember, error) {
	var member ClassMember
	filter := bson.M{
		"class_id": classID,
		"user_id":  userID,
	}

	err := m.conn.FindOneNoCache(ctx, &member, filter)
	if err != nil {
		switch {
		case errors.Is(err, monc.ErrNotFound):
			return nil, consts.ErrNotFound
		default:
			return nil, err
		}
	}

	return &member, nil
}

func (m *MemberMongoMapper) FindByMemberID(ctx context.Context, memberID string) (*ClassMember, error) {
	var member ClassMember
	oid, err := primitive.ObjectIDFromHex(memberID)
	if err != nil {
		return nil, consts.ErrInvalidObjectId
	}
	filter := bson.M{
		"_id": oid,
	}

	err = m.conn.FindOneNoCache(ctx, &member, filter)
	if err != nil {
		switch {
		case errors.Is(err, monc.ErrNotFound):
			return nil, consts.ErrNotFound
		default:
			return nil, err
		}
	}

	return &member, nil
}

func (m *MemberMongoMapper) Delete(ctx context.Context, id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return consts.ErrInvalidObjectId
	}

	_, err = m.conn.DeleteOneNoCache(ctx, bson.M{"_id": oid})
	return err
}

func (m *MemberMongoMapper) UpdateFields(ctx context.Context, id primitive.ObjectID, fields bson.M) error {
	if fields == nil {
		fields = bson.M{}
	}
	fields["update_time"] = time.Now()

	_, err := m.conn.UpdateByIDNoCache(ctx, id, bson.M{"$set": fields})
	return err
}
