package service

import (
	"context"
	"essay-show/biz/adaptor"
	"essay-show/biz/application/dto/essay/show"
	"essay-show/biz/infrastructure/consts"
	"essay-show/biz/infrastructure/repository/class"
	"essay-show/biz/infrastructure/repository/user"
	"essay-show/biz/infrastructure/util"
	"essay-show/biz/infrastructure/util/log"
	"time"

	"github.com/google/wire"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type IClassService interface {
	CreateClass(ctx context.Context, req *show.CreateClassReq) (*show.CreateClassResp, error)
	ListClasses(ctx context.Context, req *show.ListClassesReq) (*show.ListClassesResp, error)
	GetClassMembers(ctx context.Context, req *show.GetClassMembersReq) (*show.GetClassMembersResp, error)
	CreateClassMembers(ctx context.Context, req *show.CreateClassMembersReq) (*show.CreateClassMembersResp, error)
	BindClassMember(ctx context.Context, req *show.BindClassMemberReq) (*show.Response, error)
	UnbindClassMember(ctx context.Context, req *show.UnbindClassMemberReq) (*show.Response, error)
	EditClassMemberName(ctx context.Context, req *show.EditClassMemberNameReq) (*show.Response, error)
	DeleteClassMember(ctx context.Context, req *show.DeleteClassMemberReq) (*show.Response, error)
	GetClassMemberInfo(ctx context.Context, req *show.GetClassMemberInfoReq) (*show.GetClassMemberInfoResp, error)
}

type ClassService struct {
	ClassMapper  *class.MongoMapper
	MemberMapper *class.MemberMongoMapper
	UserMapper   *user.MongoMapper
}

var ClassServiceSet = wire.NewSet(
	wire.Struct(new(ClassService), "*"),
	wire.Bind(new(IClassService), new(*ClassService)),
)

func (s *ClassService) CreateClass(ctx context.Context, req *show.CreateClassReq) (*show.CreateClassResp, error) {
	userMeta := adaptor.ExtractUserMeta(ctx)
	if userMeta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}

	user, err := s.UserMapper.FindOne(ctx, userMeta.GetUserId())
	if err != nil {
		log.Error("获取用户信息失败: %v, userID: %s", err, userMeta.GetUserId())
		return nil, consts.ErrNotFound
	}
	if user.Role != consts.RoleTeacher {
		return nil, consts.ErrNotAuthentication
	}

	// 创建班级
	now := time.Now()
	c := &class.Class{
		Name:        req.Name,
		Description: req.Description,
		CreatorID:   userMeta.GetUserId(),
		MemberCount: 0, // 不算老师自己
		CreateTime:  now,
		UpdateTime:  now,
	}

	err = s.ClassMapper.Insert(ctx, c)
	if err != nil {
		log.Error("创建班级失败: %v", err)
		return nil, consts.ErrCreateClass
	}

	return &show.CreateClassResp{
		ClassId: c.ID.Hex(),
	}, nil
}

func (s *ClassService) ListClasses(ctx context.Context, req *show.ListClassesReq) (*show.ListClassesResp, error) {
	userMeta := adaptor.ExtractUserMeta(ctx)
	if userMeta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}

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

	user, err := s.UserMapper.FindOne(ctx, userMeta.GetUserId())
	if err != nil {
		log.Error("获取用户信息失败: %v", err)
		return nil, consts.ErrNotFound
	}

	// 获取老师班级
	if user.Role == consts.RoleTeacher {
		classes, total, err := s.ClassMapper.FindByCreator(ctx, userMeta.GetUserId(), page, pageSize)
		if err != nil {
			log.Error("获取班级列表失败: %v", err)
			return nil, consts.ErrGetClassList
		}

		classInfos := make([]*show.ClassInfo, 0, len(classes))
		for _, c := range classes {
			user, err := s.UserMapper.FindOne(ctx, c.CreatorID)
			if err != nil {
				log.Error("获取用户信息失败: %v", err)
				continue
			}

			classInfos = append(classInfos, &show.ClassInfo{
				Id:          c.ID.Hex(),
				Name:        c.Name,
				Description: c.Description,
				MemberCount: c.MemberCount,
				CreateTime:  c.CreateTime.Unix(),
				CreatorId:   c.CreatorID,
				CreatorName: user.Username,
			})
		}
		return &show.ListClassesResp{
			Classes: classInfos,
			Total:   total,
		}, nil
	}

	// 获取学生班级
	members, total, err := s.MemberMapper.FindByStuID(ctx, userMeta.GetUserId())
	if err != nil {
		log.Error("获取学生班级失败: %v", err)
		return nil, consts.ErrGetClassList
	}

	classInfos := make([]*show.ClassInfo, 0, len(members))
	for _, m := range members {
		c, err := s.ClassMapper.FindOne(ctx, m.ClassID)
		if err != nil {
			log.Error("获取班级信息失败: %v, classID: %v", err, m.ClassID)
			continue
		}
		user, err := s.UserMapper.FindOne(ctx, c.CreatorID)
		if err != nil {
			log.Error("获取用户信息失败: %v, createID: %v", err, c.CreatorID)
			continue
		}
		classInfos = append(classInfos, &show.ClassInfo{
			Id:          c.ID.Hex(),
			Name:        c.Name,
			Description: c.Description,
			MemberCount: c.MemberCount,
			CreateTime:  c.CreateTime.Unix(),
			CreatorId:   c.CreatorID,
			CreatorName: user.Username,
		})
	}

	return &show.ListClassesResp{
		Classes: classInfos,
		Total:   total,
	}, nil
}

func (s *ClassService) CreateClassMembers(ctx context.Context, req *show.CreateClassMembersReq) (*show.CreateClassMembersResp, error) {
	meta := adaptor.ExtractUserMeta(ctx)
	if meta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}
	if len(req.Names) == 0 {
		return nil, consts.ErrInvalidParams
	}

	success := make([]bool, len(req.Names))
	newMemberCount := int64(0)

	for i, name := range req.Names {
		existingMember, err := s.MemberMapper.FindByClassIDAndName(ctx, req.ClassId, name)
		if err == nil && existingMember != nil {
			success[i] = true
			continue
		}

		member := &class.ClassMember{
			ClassID: req.ClassId,
			Name:    name,
		}
		err = s.MemberMapper.Insert(ctx, member)
		if err != nil {
			log.Error("创建班级成员 %s 失败: %v", name, err)
			success[i] = false
		} else {
			success[i] = true
			newMemberCount++
		}
	}

	if newMemberCount > 0 {
		err := s.ClassMapper.UpdateMemberCount(ctx, req.ClassId, newMemberCount)
		if err != nil {
			log.Error("更新班级成员数量失败: %v", err)
		}
	}

	return &show.CreateClassMembersResp{
		Success: success,
	}, nil
}

// GetClassMembers 获取班级成员
func (s *ClassService) GetClassMembers(ctx context.Context, req *show.GetClassMembersReq) (*show.GetClassMembersResp, error) {
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

	// 获取班级成员
	members, total, err := s.MemberMapper.FindByClassID(ctx, req.ClassId, page, pageSize)
	if err != nil {
		log.Error("获取班级成员失败: %v", err)
		return nil, consts.ErrGetClassMembers
	}

	memberInfos := make([]*show.ClassMemberInfo, 0, len(members))
	for _, m := range members {
		memberInfo := &show.ClassMemberInfo{
			MemberId: m.ID.Hex(),
			Name:     m.Name,
			UserId:   m.UserID,
			JoinTime: func() *int64 {
				if m.JoinTime != nil {
					joinTime := m.JoinTime.Unix()
					return &joinTime
				}
				return nil
			}(),
		}
		memberInfos = append(memberInfos, memberInfo)
	}

	return &show.GetClassMembersResp{
		Members: memberInfos,
		Total:   total,
	}, nil
}

func (s *ClassService) BindClassMember(ctx context.Context, req *show.BindClassMemberReq) (*show.Response, error) {
	meta := adaptor.ExtractUserMeta(ctx)
	if meta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}
	userID := meta.GetUserId()

	// 确认学生身份
	u, err := s.UserMapper.FindOne(ctx, userID)
	if err != nil {
		log.Error("获取用户信息失败: %v", err)
		return nil, consts.ErrNotFound
	}
	if u.Role != consts.RoleStudent {
		return nil, consts.ErrNotAuthentication
	}

	// 检查是否已经是班级成员且成员没被绑定
	existingStudent, err1 := s.MemberMapper.FindByClassIDAndStuID(ctx, req.ClassId, userID)
	existingMember, err2 := s.MemberMapper.FindByMemberID(ctx, req.MemberId)

	switch {
	case err1 == nil && err2 == nil:
		if existingStudent.ID == existingMember.ID && existingMember.UserID != nil && *existingMember.UserID == userID {
			// 学生已经绑定到这个member上了
			return util.Succeed("已进入班级")
		}
		if req.MemberId != existingStudent.ID.Hex() {
			// 学生已绑定到其他member上
			return nil, consts.ErrMemberAlreadyBound
		}
		if existingMember.UserID != nil && *existingMember.UserID != userID {
			// 指定的member已被其他学生绑定
			return nil, consts.ErrMemberPositionOccupied
		}
		// 理论上不应该到这里，但为了安全起见
		return nil, consts.ErrBindClassMember

	case err1 == nil && err2 != nil:
		// 学生已在班级中，但指定的member不存在
		return nil, consts.ErrMemberPositionNotFound

	case err1 != nil && err2 == nil:
		// 学生不在班级中，指定的member已被其他人绑定
		if existingMember.UserID != nil {
			return nil, consts.ErrMemberPositionOccupied
		}
		// 指定的member不属于当前班级
		if existingMember.ClassID != req.ClassId {
			return nil, consts.ErrMemberPositionNotFound
		}
		// member未绑定，可以绑定
		updateFields := bson.M{
			"user_id":   userID,
			"join_time": time.Now(),
		}
		if err := s.MemberMapper.UpdateFields(ctx, existingMember.ID, updateFields); err != nil {
			log.Error("绑定班级成员失败: %v", err)
			return nil, consts.ErrBindClassMember
		}
		return util.Succeed("绑定成功")

	case err1 != nil && err2 != nil:
		// 学生不在班级中，指定的member也不存在
		return nil, consts.ErrMemberPositionNotFound

	default:
		// 理论上不会到这里
		return nil, consts.ErrBindClassMember
	}
}

func (s *ClassService) UnbindClassMember(ctx context.Context, req *show.UnbindClassMemberReq) (*show.Response, error) {
	meta := adaptor.ExtractUserMeta(ctx)
	if meta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}
	userID := meta.GetUserId()

	// 确认学生身份
	u, err := s.UserMapper.FindOne(ctx, userID)
	if err != nil {
		log.Error("获取用户信息失败: %v", err)
		return nil, consts.ErrNotFound
	}
	if u.Role != consts.RoleStudent {
		return nil, consts.ErrNotAuthentication
	}

	updateFields := bson.M{
		"user_id":   nil,
		"join_time": nil,
	}

	oid, err := primitive.ObjectIDFromHex(req.MemberId)
	if err != nil {
		return nil, consts.ErrInvalidObjectId
	}

	err = s.MemberMapper.UpdateFields(ctx, oid, updateFields)
	if err != nil {
		return nil, err
	}
	return util.Succeed("解绑成功")
}

func (s *ClassService) EditClassMemberName(ctx context.Context, req *show.EditClassMemberNameReq) (*show.Response, error) {
	meta := adaptor.ExtractUserMeta(ctx)
	if meta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}

	updateFields := bson.M{
		"name": req.Name,
	}

	oid, err := primitive.ObjectIDFromHex(req.MemberId)
	if err != nil {
		return nil, consts.ErrInvalidObjectId
	}

	err = s.MemberMapper.UpdateFields(ctx, oid, updateFields)
	if err != nil {
		return nil, err
	}
	return util.Succeed("修改成功")
}

func (s *ClassService) DeleteClassMember(ctx context.Context, req *show.DeleteClassMemberReq) (*show.Response, error) {
	meta := adaptor.ExtractUserMeta(ctx)
	if meta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}
	userID := meta.GetUserId()

	// 确认教师身份
	u, err := s.UserMapper.FindOne(ctx, userID)
	if err != nil {
		log.Error("获取用户信息失败: %v", err)
		return nil, consts.ErrNotFound
	}
	if u.Role != consts.RoleTeacher {
		return nil, consts.ErrNotAuthentication
	}

	err = s.MemberMapper.Delete(ctx, req.MemberId)
	if err != nil {
		return nil, err
	}
	member, err := s.MemberMapper.FindByMemberID(ctx, req.MemberId)
	if err != nil {
		return nil, err
	}
	err = s.ClassMapper.UpdateMemberCount(ctx, member.ClassID, -1)
	if err != nil {
		return nil, err
	}
	// 删除成员作业 TODO
	return util.Succeed("删除成功")
}

func (s *ClassService) GetClassMemberInfo(ctx context.Context, req *show.GetClassMemberInfoReq) (*show.GetClassMemberInfoResp, error) {
	meta := adaptor.ExtractUserMeta(ctx)
	if meta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}
	userID := meta.GetUserId()

	// 确认学生身份
	u, err := s.UserMapper.FindOne(ctx, userID)
	if err != nil {
		log.Error("获取用户信息失败: %v, userID: %s", err, userID)
		return nil, consts.ErrNotFound
	}
	if u.Role != consts.RoleStudent {
		return nil, consts.ErrNotAuthentication
	}

	member, err := s.MemberMapper.FindByClassIDAndStuID(ctx, req.ClassId, userID)
	if err != nil {
		log.Error("获取班级成员信息失败: %v, classID: %s, userID: %s", err, req.ClassId, userID)
		return nil, consts.ErrNotFound
	}
	return &show.GetClassMemberInfoResp{
		Name:     member.Name,
		MemberId: member.ID.Hex(),
		JoinTime: member.JoinTime.Unix(),
	}, nil
}
