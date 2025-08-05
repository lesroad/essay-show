package question_bank

import (
	"essay-show/biz/infrastructure/config"
	"essay-show/biz/infrastructure/util/log"
)

// NewMySQLMapperFromConfig 创建 MySQL 映射器
func NewMySQLMapperFromConfig(config *config.Config) (*MySQLMapper, error) {
	log.Info("Creating MySQL mapper with DSN: %s", config.MySQL.DSN)
	return NewMySQLMapper(config.MySQL.DSN)
}
