package redis

import (
	"essay-show/biz/infrastructure/config"
	"sync"

	"github.com/zeromicro/go-zero/core/stores/redis"
)

// Redis连接管理
// 提供统一的Redis客户端实例

var instance *redis.Redis
var once sync.Once

// GetRedis 构造一个Redis客户端
func GetRedis(config *config.Config) *redis.Redis {
	once.Do(func() {
		instance = redis.MustNewRedis(*config.Redis)
	})
	return instance
}
