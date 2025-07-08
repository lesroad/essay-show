package lock

import (
	"context"
	"errors"
	"essay-show/biz/infrastructure/config"
	"essay-show/biz/infrastructure/redis"
	"fmt"
	"time"

	"github.com/google/uuid"
	gozero_redis "github.com/zeromicro/go-zero/core/stores/redis"
)

// 分布式锁实现
// 基于Redis实现的分布式锁，支持自动续期和Watch Dog机制
// 分布式锁会设定初始有效时间, 最长有效时间
// 在本项目中该分布式锁的应用是用户批改时会先尝试获取对应的锁, 获取失败则直接返回有一个批改进程
// 若获取成功, 则进入处理流程, 同时会启动一个watch dog线程, 用于锁的生命周期管理
// watch dog 发现锁过期但处理函数仍未结束时, 自动续期
// 处理函数结束时, 通知watch dog释放锁, 若超时, 则自动释放锁

// IDistributedLock 分布式锁接口
type IDistributedLock interface {
	Lock() error
	Unlock() error
	Expired() bool
}

// EvaMutex 批改分布式锁
type EvaMutex struct {
	rds *gozero_redis.Redis
	// key 键, 获取资源的标识
	key string
	// value 需要是唯一标识, 避免锁错误释放
	value string
	// ctx 上下文
	ctx context.Context
	// cancel 取消函数
	cancel context.CancelFunc
	// expire 有效时长
	expire int
	// start 获取到锁的时间
	start time.Time
	// ttl 最长存活时间
	ttl int
	// isExpired 是否过期
	isExpired bool
}

// retries 默认重试次数3
var retries = 3

// renewScript 锁续期脚本
const renewScript = `if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("EXPIRE", KEYS[1], ARGV[2])
else
    return 0
end`

// unlockScript 释放锁脚本
const unlockScript = `if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
else
    return 0
end`

// NewEvaMutex 创建一个新的Redis分布式锁
func NewEvaMutex(c context.Context, key string, expire, ttl int) *EvaMutex {
	ctx, cancel := context.WithCancel(c)
	return &EvaMutex{
		rds:       redis.GetRedis(config.GetConfig()),
		key:       key,
		value:     uuid.New().String(),
		ctx:       ctx,
		cancel:    cancel,
		expire:    expire,
		ttl:       ttl,
		isExpired: false,
	}
}

// Lock 加锁
func (e *EvaMutex) Lock() error {
	for i := 0; i < retries; i++ {
		ok, err := e.rds.SetnxExCtx(e.ctx, e.key, e.value, e.expire)
		if err != nil || !ok {
			// 这里不用指数退避而是默认1s是因为为了避免用户等待过长时间
			time.Sleep(1 * time.Second)
			continue
		}
		e.start = time.Now()
		go e.watchDog()
		return nil
	}
	return errors.New("获取锁失败")
}

// Unlock 释放锁
func (e *EvaMutex) Unlock() (err error) {
	// 停止watch dog
	e.cancel()
	// 释放锁
	for i := 0; i < retries; i++ {
		_, err = e.rds.EvalCtx(context.Background(), unlockScript, []string{e.key}, e.value)
		if err == nil {
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("释放锁失败: %v", err)
}

// Expired 返回是否超时
func (e *EvaMutex) Expired() bool {
	return e.isExpired
}

// watchDog 看门狗, 实现自动续期
func (e *EvaMutex) watchDog() {
	// 初始有效时间计时器
	ticker := time.NewTicker(time.Duration(e.expire) * time.Second / 2)
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			if time.Since(e.start) > time.Duration(e.ttl)*time.Second {
				e.isExpired = true
				e.cancel()
				return
			}
			if err := e.renew(); err != nil {
				e.cancel()
				return
			}
			ticker.Reset(time.Duration(e.expire) * time.Second / 2)
		}
	}
}

// renew 锁续期
func (e *EvaMutex) renew() error {
	e.reExpire()
	val, err := e.rds.EvalCtx(e.ctx, renewScript, []string{e.key}, e.value, e.expire)
	if err != nil {
		return fmt.Errorf("续期请求失败: %w", err)
	}
	if success, _ := val.(int64); success != 1 {
		return errors.New("续期失败，锁可能已丢失")
	}
	return err
}

// reExpire 更新剩余有效期
// 目前这里的计算方式没有实际的依据, 估计一篇修改需要16s, 初始有效期设置24s, 最长存活40s
// watch dog第一次休眠12秒, 第二次休眠6秒, 此后每次5s直到超过ttl
// 第二次休眠前大概率就释放锁了, 如果是没有, 那么大概率是算法端导致的阻塞, 等到40s后杀死线程
func (e *EvaMutex) reExpire() {
	e.expire = e.expire / 2
	if e.expire < 5 {
		e.expire = 5
	}
}
