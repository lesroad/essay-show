# Essay Show - 作文批改系统

基于 Go-Zero 框架开发的作文智能批改系统，采用 DDD 分层架构设计，支持实时作文批改、PDF下载、缓存优化等功能。

## 📁 项目结构

```
essay-show/
├── biz/                                    # 业务逻辑层
│   ├── adaptor/                           # 适配器层
│   │   ├── controller/                    # 控制器
│   │   ├── router/                        # 路由配置
│   │   └── extract.go                     # 用户信息提取
│   ├── application/                       # 应用服务层
│   │   ├── dto/                          # 数据传输对象
│   │   │   ├── basic/                    # 基础DTO
│   │   │   └── essay/                    # 作文相关DTO
│   │   └── service/                      # 应用服务
│   │       ├── essay.go                  # 作文服务
│   │       ├── user.go                   # 用户服务
│   │       └── sts.go                    # 临时凭证服务
│   └── infrastructure/                    # 基础设施层
│       ├── config/                       # 配置管理
│       ├── consts/                       # 常量定义
│       ├── repository/                   # 数据仓储层
│       │   ├── user/                     # 用户数据访问
│       │   ├── log/                      # 日志数据访问
│       │   ├── exercise/                 # 练习数据访问
│       │   ├── attend/                   # 签到数据访问
│       │   ├── invitation/               # 邀请数据访问
│       │   └── feedback/                 # 反馈数据访问
│       ├── cache/                        # 缓存层
│       │   └── download_cache.go         # 下载缓存服务
│       ├── lock/                         # 分布式锁层
│       │   └── distributed_lock.go       # 分布式锁服务
│       ├── redis/                        # Redis基础设施
│       │   └── redis.go                  # Redis连接管理
│       └── util/                         # 工具类
├── provider/                              # 依赖注入
│   ├── provider.go                       # 依赖配置
│   ├── wire.go                           # Wire配置
│   └── wire_gen.go                       # Wire生成代码
└── main.go                               # 程序入口
```

## 🏗️ DDD 分层架构

### 1. 表示层 (Presentation Layer)
- **位置**: `biz/adaptor/`
- **职责**: 处理HTTP请求响应、路由配置、参数验证
- **组件**: Controller、Router、Middleware

### 2. 应用层 (Application Layer)
- **位置**: `biz/application/service/`
- **职责**: 业务逻辑编排、事务管理、缓存策略
- **特点**: 不包含业务规则，只负责协调领域对象

### 3. 基础设施层 (Infrastructure Layer)
- **位置**: `biz/infrastructure/`
- **职责**: 技术实现细节，数据持久化、缓存、外部API调用
- **组件**: 
  - `repository/`: 数据仓储（MongoDB操作）
  - `cache/`: 缓存服务（Redis缓存）
  - `lock/`: 分布式锁服务（Redis锁）
  - `redis/`: Redis基础设施（连接管理）
  - `util/`: 技术工具类

### 4. 依赖注入层
- **位置**: `provider/`
- **职责**: 管理对象生命周期和依赖关系
- **工具**: Google Wire

## 🔧 核心功能

### 1. 作文批改
- **同步批改**: `EssayEvaluate` - 传统的同步批改接口
- **流式批改**: `EssayEvaluateStream` - 支持实时进度反馈的流式批改
- **结果下载**: `DownloadEvaluate` - 批改结果PDF下载（支持缓存）

### 2. 缓存优化 🚀
#### Redis 缓存架构
```go
// 缓存接口抽象
type IDownloadCacheMapper interface {
    Get(ctx context.Context, id string) (*show.DownloadEvaluateResp, error)
    Set(ctx context.Context, id string, data *show.DownloadEvaluateResp) error
    Delete(ctx context.Context, id string) error
}

// 缓存实现
type DownloadCacheMapper struct {
    rds *gozero_redis.Redis
}
```

#### 缓存策略
- **缓存Key**: `download_evaluate:{id}`
- **过期时间**: 1小时 (3600秒)
- **缓存场景**: PDF下载链接缓存，避免重复调用下游API
- **降级策略**: 缓存失败不影响正常业务流程

### 3. 分布式锁 🔐
#### 分布式锁架构
```go
// 分布式锁实现（位于lock包中）
type EvaMutex struct {
    rds *gozero_redis.Redis  // 通过redis.GetRedis()获取连接
    key string
    value string    // 唯一标识，防止误释放
    // ... 其他字段
}

// 使用方式
key := "evaluate" + userId
distributedLock := lock.NewEvaMutex(ctx, key, 20, 40)
if err := distributedLock.Lock(); err != nil {
    // 处理获取锁失败
}
defer distributedLock.Unlock()
```

#### 锁特性
- **场景**: 防止用户同时发起多个批改请求
- **实现**: 基于Redis的分布式锁，支持自动续期
- **特性**: Watch Dog机制、超时自动释放、唯一标识防误释放
- **位置**: 独立的`lock`包，按业务功能组织

## 🛠️ 技术栈

- **框架**: Go-Zero (微服务框架)
- **数据库**: MongoDB
- **缓存**: Redis
- **依赖注入**: Google Wire
- **链路追踪**: OpenTelemetry + Jaeger
- **日志**: go-zero 内置日志系统

## 🚀 快速开始

### 1. 环境要求
- Go 1.19+
- MongoDB 4.4+
- Redis 6.0+

### 2. 安装依赖
```bash
go mod download
```

### 3. 配置文件
复制并修改配置文件：
```bash
cp biz/infrastructure/config/config.local.yaml config.yaml
```

### 4. 生成依赖注入代码
```bash
cd provider && wire
```

### 5. 启动服务
```bash
go run main.go
```

## 🔧 开发指南

### 1. 添加新的缓存
遵循DDD分层原则，在 `infrastructure/cache/` 中创建新的缓存服务：

```go
// 1. 定义接口
type INewCacheService interface {
    Get(ctx context.Context, key string) (*DataType, error)
    Set(ctx context.Context, key string, data *DataType) error
}

// 2. 实现接口
type NewCacheService struct {
    rds *gozero_redis.Redis
}

// 3. 注册到依赖注入
// 在 provider/provider.go 的 InfrastructureSet 中添加
```

### 2. 添加新的数据仓储
在 `infrastructure/repository/` 中创建新的数据访问层：

```go
// 1. 定义接口
type INewRepository interface {
    Insert(ctx context.Context, data *Model) error
    FindOne(ctx context.Context, id string) (*Model, error)
}

// 2. 实现接口（MongoDB）
type NewMongoMapper struct {
    conn *monc.Model
}

// 3. 注册到依赖注入
func NewNewMongoMapper(config *config.Config) *NewMongoMapper {
    conn := monc.MustNewModel(config.Mongo.URL, config.Mongo.DB, "collection", config.Cache)
    return &NewMongoMapper{conn: conn}
}
```

### 3. 使用分布式锁
```go
// 在Service中使用分布式锁
import "essay-show/biz/infrastructure/lock"

key := "operation_" + userId
distributedLock := lock.NewEvaMutex(ctx, key, initialExpire, maxTTL)
if err := distributedLock.Lock(); err != nil {
    return consts.ErrLockFailed
}
defer func() {
    if err := distributedLock.Unlock(); err != nil || distributedLock.Expired() {
        logx.Error("unlock failed: %v, expired: %v", err, distributedLock.Expired())
    }
}()
// 执行需要加锁的业务逻辑
```

### 4. Wire 依赖注入
修改依赖后，需要重新生成：
```bash
cd provider && wire
```

### 5. 代码规范
- 遵循DDD分层架构原则
- Service层不直接操作基础设施
- 通过接口抽象降低耦合
- Repository层专注数据访问
- Cache层专注缓存逻辑
- Lock层专注分布式锁逻辑
- Redis层提供基础设施连接
- 统一错误处理和日志记录

## 📊 性能优化

### 1. 缓存优化
- **下载链接缓存**: 1小时内重复请求直接返回缓存结果
- **响应时间**: 从几百毫秒降至几毫秒
- **API调用**: 减少下游服务压力

### 2. 分布式锁优化
- **并发控制**: 防止用户重复提交，减少资源竞争
- **自动续期**: Watch Dog机制确保长时间操作不会锁超时
- **性能监控**: 锁获取和释放的详细日志
- **架构清晰**: 锁功能独立于缓存和连接管理，按业务功能组织

### 3. 链路追踪
- 集成OpenTelemetry，支持分布式链路追踪
- 可视化性能瓶颈定位

## 🔒 安全特性

- **JWT认证**: 用户身份验证
- **权限验证**: 确保用户只能访问自己的数据
- **参数校验**: 请求参数安全验证
- **分布式锁**: 防止并发操作冲突
- **缓存隔离**: 用户数据缓存隔离

## 📝 API文档

服务启动后访问: `http://localhost:8888/swagger`

## 🎯 架构优势

### 1. 符合DDD原则
- **Repository模式**: 数据访问层抽象，易于测试和替换
- **Cache层分离**: 缓存逻辑独立，职责单一
- **Lock层独立**: 分布式锁逻辑独立，按业务功能组织
- **Redis层基础**: 提供统一的Redis连接基础设施
- **清晰分层**: 每层职责明确，依赖关系清晰

### 2. 可维护性
- **分层清晰**: 各层职责单一，易于理解和维护
- **低耦合**: 通过接口抽象，层与层之间松耦合
- **高内聚**: 相关功能聚合在同一模块内
- **避免重复**: 移除冗余的锁服务，保持代码简洁

### 3. 可测试性
- **接口抽象**: 易于mock和单元测试
- **依赖注入**: 测试时可替换依赖
- **分层隔离**: 可以独立测试各层

### 4. 可扩展性
- **插件化**: 新功能可以通过新的service扩展
- **技术栈替换**: 基础设施层可以无缝替换技术实现
- **微服务友好**: 易于拆分为独立的微服务

## 🤝 贡献指南

1. Fork 项目
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 创建 Pull Request

## 📄 许可证

本项目采用 MIT 许可证 - 详见 [LICENSE](LICENSE) 文件

---

**如有问题，欢迎提交 Issue 或 PR！** 🎉
