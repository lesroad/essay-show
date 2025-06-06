package adaptor

import (
	"context"
	"encoding/json"
	"errors"
	"essay-show/biz/application/dto/basic"
	"essay-show/biz/infrastructure/config"
	"essay-show/biz/infrastructure/consts"
	"essay-show/biz/infrastructure/util"
	"essay-show/biz/infrastructure/util/log"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/golang-jwt/jwt/v4"
)

const hertzContext = "hertz_context"

func InjectContext(ctx context.Context, c *app.RequestContext) context.Context {
	return context.WithValue(ctx, hertzContext, c)
}

func ExtractContext(ctx context.Context) (*app.RequestContext, error) {
	c, ok := ctx.Value(hertzContext).(*app.RequestContext)
	if !ok {
		return nil, errors.New("hertz context not found")
	}
	return c, nil
}

func ExtractUserMeta(ctx context.Context) (user *basic.UserMeta) {
	user = new(basic.UserMeta)
	var err error
	defer func() {
		if err != nil {
			log.CtxInfo(ctx, "extract user meta fail, err=%v", err)
		}
	}()
	c, err := ExtractContext(ctx)
	if err != nil {
		return
	}
	tokenString := c.GetHeader("Authorization")
	token, err := jwt.Parse(string(tokenString), func(_ *jwt.Token) (interface{}, error) {
		return jwt.ParseECPublicKeyFromPEM([]byte(config.GetConfig().Auth.PublicKey))
	})
	if err != nil {
		return
	}
	if !token.Valid {
		err = errors.New("token is not valid")
		return
	}
	data, err := json.Marshal(token.Claims)
	if err != nil {
		return
	}
	err = json.Unmarshal(data, user)
	if err != nil {
		return
	}
	if user.SessionUserId == "" {
		user.SessionUserId = user.UserId
	}
	if user.SessionAppId == 0 {
		user.SessionAppId = user.AppId
	}
	if user.SessionDeviceId == "" {
		user.SessionDeviceId = user.DeviceId
	}
	log.CtxInfo(ctx, "userMeta=%s", util.JSONF(user))
	return
}

// generateJwtToken 生成jwt
/*
生成 ECDSA 私钥: openssl ecparam -genkey -name prime256v1 -noout -out private_key.pem
从私钥中提取公钥: openssl ec -in private_key.pem -pubout -out public_key.pem
*/
func GenerateJwtToken(resp map[string]any) (string, int64, error) {
	key, err := jwt.ParseECPrivateKeyFromPEM([]byte(config.GetConfig().Auth.SecretKey))
	if err != nil {
		return "", 0, err
	}
	iat := time.Now().Unix()
	exp := iat + config.GetConfig().Auth.AccessExpire
	claims := make(jwt.MapClaims)
	claims["exp"] = exp
	claims["iat"] = iat
	claims["userId"] = resp["userId"].(string)
	claims["appId"] = consts.AppId
	claims["deviceId"] = "" // 暂时传空
	// claims["wechatUserMeta"] = &basic.WechatUserMeta{
	// 	AppId:   resp["appId"].(string),
	// 	OpenId:  resp["openId"].(string),
	// 	UnionId: resp["unionId"].(string),
	// }
	token := jwt.New(jwt.SigningMethodES256)
	token.Claims = claims
	tokenString, err := token.SignedString(key)
	if err != nil {
		return "", 0, err
	}
	return tokenString, exp, nil
}
