package cache

import (
	"context"
	"encoding/json"
	"essay-show/biz/application/dto/essay/show"
	"essay-show/biz/infrastructure/config"
	"essay-show/biz/infrastructure/redis"
	"fmt"

	gozero_redis "github.com/zeromicro/go-zero/core/stores/redis"
)

const (
	downloadEvaluateCachePrefix = "download_evaluate"
	downloadEvaluateCacheExpire = 3600 // 1小时
)

type IDownloadCacheMapper interface {
	Get(ctx context.Context, id string) (*show.DownloadEvaluateResp, error)
	Set(ctx context.Context, id string, data *show.DownloadEvaluateResp) error
	Delete(ctx context.Context, id string) error
}

type DownloadCacheMapper struct {
	rds *gozero_redis.Redis
}

func NewDownloadCacheMapper(config *config.Config) *DownloadCacheMapper {
	return &DownloadCacheMapper{
		rds: redis.GetRedis(config),
	}
}

// Get 从缓存获取下载评估结果
func (m *DownloadCacheMapper) Get(ctx context.Context, id string) (*show.DownloadEvaluateResp, error) {
	cacheKey := m.buildCacheKey(id)

	cachedData, err := m.rds.GetCtx(ctx, cacheKey)
	if err != nil {
		return nil, err
	}

	if cachedData == "" {
		return nil, fmt.Errorf("cache miss")
	}

	var result show.DownloadEvaluateResp
	if err := json.Unmarshal([]byte(cachedData), &result); err != nil {
		return nil, fmt.Errorf("unmarshal cached data failed: %w", err)
	}

	return &result, nil
}

// Set 将下载评估结果存入缓存
func (m *DownloadCacheMapper) Set(ctx context.Context, id string, data *show.DownloadEvaluateResp) error {
	cacheKey := m.buildCacheKey(id)

	resultBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal data failed: %w", err)
	}

	return m.rds.SetexCtx(ctx, cacheKey, string(resultBytes), downloadEvaluateCacheExpire)
}

// Delete 删除缓存
func (m *DownloadCacheMapper) Delete(ctx context.Context, id string) error {
	cacheKey := m.buildCacheKey(id)
	_, err := m.rds.DelCtx(ctx, cacheKey)
	return err
}

// buildCacheKey 构造缓存key
func (m *DownloadCacheMapper) buildCacheKey(id string) string {
	return fmt.Sprintf("%s:%s", downloadEvaluateCachePrefix, id)
}
