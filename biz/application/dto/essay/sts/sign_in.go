package sts

type SignInResp struct {
	UserId  string  `protobuf:"bytes,1,opt,name=userId,proto3" form:"userId" json:"userId" query:"userId"`
	OpenId  string  `protobuf:"bytes,2,opt,name=openId,proto3" form:"openId" json:"openId" query:"openId"`
	UnionId string  `protobuf:"bytes,3,opt,name=unionId,proto3" form:"unionId" json:"unionId" query:"unionId"`
	AppId   string  `protobuf:"bytes,4,opt,name=appId,proto3" form:"appId" json:"appId" query:"appId"`
	Options *string `protobuf:"bytes,5,opt,name=options,proto3,oneof" form:"options" json:"options" query:"options"` // 可选参数
}
