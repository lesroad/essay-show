package config

import (
	_ "embed"
	"essay-show/biz/infrastructure/util/log"
	"os"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

// //go:embed config.local.yaml
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
	MySQL struct {
		DSN string
	}
	Cache cache.CacheConf
	Redis *redis.RedisConf
	Api   API
	Log   LogConfig
}

type LogConfig struct {
	NoLogPaths []string
}

type API struct {
	PlatfromURL          string
	MiniProgramURL       string
	GenerateExercisesURL string
	TitleUrlOcr          string
	EvaluateUrl          string
	DownloadURL          string
	EssayInfoURL         string
}

func NewConfig() (*Config, error) {
	c := new(Config)

	if len(embeddedConfig) == 0 {
		path := os.Getenv("CONFIG_PATH")
		log.Info("NewConfig load config from path: %s", path)
		err := conf.Load(path, c)
		if err != nil {
			return nil, err
		}
	} else {
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
