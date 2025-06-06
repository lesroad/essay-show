package config

import (
	_ "embed"
	"os"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

//go:embed config.local.yaml
var embeddedConfig []byte

var config *Config

type Auth struct {
	SecretKey    string
	PublicKey    string
	AccessExpire int64
}

type Config struct {
	service.ServiceConf
	ListenOn string
	State    string
	Auth     Auth
	Mongo    struct {
		URL string
		DB  string
	}
	Cache cache.CacheConf
	Redis *redis.RedisConf
	Api   API
}

type API struct {
	PlatfromURL          string
	GenerateExercisesURL string
	BeeTitleUrlOcr       string
	BetaEvaluateUrl      string
}

func NewConfig() (*Config, error) {
	c := new(Config)

	// 优先使用环境变量指定的配置文件
	path := os.Getenv("CONFIG_PATH")
	if path != "" {
		err := conf.Load(path, c)
		if err != nil {
			return nil, err
		}
	} else {
		// 使用嵌入的配置文件
		err := conf.LoadFromYamlBytes(embeddedConfig, c)
		if err != nil {
			return nil, err
		}
	}

	err := c.SetUp()
	if err != nil {
		return nil, err
	}
	config = c
	return c, nil
}

func GetConfig() *Config {
	return config
}
