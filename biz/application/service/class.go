package service

import (
	"context"
	"crypto/rand"
	"essay-show/biz/adaptor"
	"essay-show/biz/application/dto/essay/show"
	"essay-show/biz/infrastructure/config"
	"essay-show/biz/infrastructure/consts"
	"essay-show/biz/infrastructure/repository/class"
	"essay-show/biz/infrastructure/repository/user"
	"essay-show/biz/infrastructure/util/log"
	"math/big"
	"time"

	"github.com/google/wire"
)

type IClassService interface {
	CreateClass(ctx context.Context, req *show.CreateClassReq) (*show.CreateClassResp, error)
	ListClasses(ctx context.Context, req *show.ListClassesReq) (*show.ListClassesResp, error)
	JoinClass(ctx context.Context, req *show.JoinClassReq) (*show.JoinClassResp, error)
	GetClassMembers(ctx context.Context, req *show.GetClassMembersReq) (*show.GetClassMembersResp, error)
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

// CreateClass 创建班级
func (s *ClassService) CreateClass(ctx context.Context, req *show.CreateClassReq) (*show.CreateClassResp, error) {
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
	if user.Role != consts.RoleTeacher {
		return nil, consts.ErrNotAuthentication
	}

	// 生成邀请码
	inviteCode := s.generateInviteCode()

	// 创建班级
	now := time.Now()
	c := &class.Class{
		Name:        req.Name,
		Description: req.Description,
		InviteCode:  inviteCode,
		CreatorID:   userMeta.GetUserId(),
		MemberCount: 1, // 创建者自动成为成员
		CreateTime:  now,
		UpdateTime:  now,
	}

	err = s.ClassMapper.Insert(ctx, c)
	if err != nil {
		log.Error("创建班级失败: %v", err)
		return nil, consts.ErrCreateClass
	}

	// 创建者自动加入班级
	member := &class.ClassMember{
		ClassID:    c.ID.Hex(),
		UserID:     userMeta.GetUserId(),
		Role:       consts.RoleTeacher,
		JoinTime:   now,
		CreateTime: now,
		UpdateTime: now,
	}

	err = s.MemberMapper.Insert(ctx, member)
	if err != nil {
		log.Error("添加班级成员失败: %v", err)
		// 这里可以考虑回滚班级创建，但为了简单起见，暂时只记录错误
	}

	return &show.CreateClassResp{
		ClassId:    c.ID.Hex(),
		InviteCode: inviteCode,
		InviteUrl:  config.GetConfig().Api.ClassJoinURL + "?invite_code=" + inviteCode,
	}, nil
}

// ListClasses 获取班级列表
func (s *ClassService) ListClasses(ctx context.Context, req *show.ListClassesReq) (*show.ListClassesResp, error) {
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
				InviteCode:  c.InviteCode,
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
	members, total, err := s.MemberMapper.FindByUserID(ctx, userMeta.GetUserId())
	if err != nil {
		log.Error("获取学生班级失败: %v", err)
		return nil, consts.ErrGetClassList
	}

	classInfos := make([]*show.ClassInfo, 0, len(members))
	for _, m := range members {
		c, err := s.ClassMapper.FindOne(ctx, m.ClassID)
		if err != nil {
			log.Error("获取班级信息失败: %v", err)
			continue
		}
		user, err := s.UserMapper.FindOne(ctx, c.CreatorID)
		if err != nil {
			log.Error("获取用户信息失败: %v", err)
			continue
		}
		classInfos = append(classInfos, &show.ClassInfo{
			Id:          c.ID.Hex(),
			Name:        c.Name,
			Description: c.Description,
			InviteCode:  c.InviteCode,
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

// JoinClass 加入班级
func (s *ClassService) JoinClass(ctx context.Context, req *show.JoinClassReq) (*show.JoinClassResp, error) {
	// 获取用户信息
	meta := adaptor.ExtractUserMeta(ctx)
	if meta.GetUserId() == "" {
		return nil, consts.ErrNotAuthentication
	}
	userID := meta.GetUserId()

	// 根据邀请码查找班级
	c, err := s.ClassMapper.FindOneByInviteCode(ctx, req.InviteCode)
	if err != nil {
		log.Error("班级不存在: %v", err)
		return nil, consts.ErrNotFound
	}

	// 检查是否已经是班级成员
	existingMember, err := s.MemberMapper.FindByClassIDAndUserID(ctx, c.ID.Hex(), userID)
	if err == nil && existingMember != nil {
		return &show.JoinClassResp{
			ClassId:   c.ID.Hex(),
			ClassName: c.Name,
		}, nil
	}

	// 添加班级成员
	now := time.Now()
	member := &class.ClassMember{
		ClassID:    c.ID.Hex(),
		UserID:     userID,
		Role:       consts.RoleStudent,
		JoinTime:   now,
		CreateTime: now,
		UpdateTime: now,
	}

	err = s.MemberMapper.Insert(ctx, member)
	if err != nil {
		log.Error("加入班级失败: %v", err)
		return nil, consts.ErrJoinClass
	}

	// 更新班级成员数量
	err = s.ClassMapper.UpdateMemberCount(ctx, c.ID.Hex(), 1)
	if err != nil {
		log.Error("更新班级成员数量失败: %v", err)
		// 不影响主流程，只记录错误
	}

	return &show.JoinClassResp{
		ClassId:   c.ID.Hex(),
		ClassName: c.Name,
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

	// 转换为响应格式
	memberInfos := make([]*show.ClassMemberInfo, 0, len(members))
	for _, m := range members {
		// 查找用户信息
		user, err := s.UserMapper.FindOne(ctx, m.UserID)
		if err != nil {
			log.Error("获取班级成员信息失败: %v", err)
			continue
		}
		memberInfos = append(memberInfos, &show.ClassMemberInfo{
			Id:       m.ID.Hex(),
			UserId:   m.UserID,
			UserName: user.Username,
			Role:     m.Role,
			JoinTime: m.JoinTime.Unix(),
		})
	}

	return &show.GetClassMembersResp{
		Members: memberInfos,
		Total:   total,
	}, nil
}

// generateInviteCode 生成邀请码
func (s *ClassService) generateInviteCode() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	code := make([]byte, 10)
	for i := range code {
		randomIndex, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		code[i] = charset[randomIndex.Int64()]
	}
	return string(code)
}
