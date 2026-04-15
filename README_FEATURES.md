# 用户中心 - 新增功能

本文档介绍了用户中心系统新增的五大核心功能。

## 📋 功能清单

1. ✅ **Kafka 消费失败重试 + 死信队列**
2. ✅ **接口限流**
3. ✅ **登录链路压测与优化**
4. ✅ **热榜缓存一致性方案**
5. ✅ **幂等键过期与补偿机制**

---

## 🔄 1. Kafka 消费失败重试 + 死信队列

### 核心特性
- 自动重试机制（最多 3 次）
- 指数退避策略：1秒 → 5秒 → 30秒
- 超过重试次数自动进入死信队列
- 完整的重试元数据追踪

### 文件结构
```
internal/worker/
├── retry_handler.go          # 重试处理器
├── consumer_group.go          # 消费者组
└── idempotent_handler.go      # 幂等处理器

examples/
└── kafka_retry_example.go     # 使用示例
```

### 快速开始
```go
// 创建重试处理器
retryHandler := worker.NewRetryableHandler(
    yourBusinessHandler,
    kafkaProducer,
    logger,
)

// 处理消息
err := retryHandler.Handle(ctx, msg)
```

### 死信队列
- 原 topic: `user.registered`
- 死信 topic: `user.registered.dlq`

---

## 🚦 2. 接口限流

### 核心特性
- 基于 Redis 的滑动窗口算法
- 支持 IP 级别和用户级别限流
- 登录接口专用防暴力破解
- 响应头返回剩余配额

### 文件结构
```
internal/web/middleware/
├── ratelimit.go               # 通用限流中间件
└── login_ratelimit.go         # 登录限流中间件

examples/
└── ratelimit_example.go       # 使用示例
```

### 限流策略

| 场景 | 限制 | 窗口期 |
|------|------|--------|
| 全局 IP | 100 次 | 1 分钟 |
| 登录 IP | 5 次 | 5 分钟 |
| 登录账号 | 3 次 | 10 分钟 |

### 快速开始
```go
// 全局限流
limiter := middleware.NewRateLimiter(
    redisClient, 
    "api-limit", 
    time.Minute, 
    100,
)
router.Use(limiter.Build())

// 登录限流
loginLimiter := middleware.NewLoginRateLimiter(redisClient)
router.POST("/login", loginLimiter.Build(), loginHandler)
```

---

## ⚡ 3. 登录链路压测与优化

### 优化方案

#### 数据库层
- ✅ 为 `email` 和 `phone` 添加索引
- ✅ 连接池配置优化
- 📝 读写分离（建议）

#### 缓存层
- ✅ 用户信息缓存
- ✅ JWT Token 缓存
- ✅ Redis Pipeline 批量操作

#### 限流保护
- ✅ IP 级别限流
- ✅ 账号级别限流
- ✅ 验证码限流

### 压测工具

#### 使用 wrk
```bash
wrk -t12 -c400 -d30s --latency \
  -s script/benchmark_login.lua \
  http://localhost:8081/user/login

# 或使用提供的脚本
bash script/benchmark_run.sh
```

#### 使用 ab
```bash
ab -n 10000 -c 100 \
  -p login.json -T application/json \
  http://localhost:8081/user/login
```

### 性能指标
- 目标 QPS: 1000+
- P99 延迟: < 100ms
- 错误率: < 0.1%

---

## 🔥 4. 热榜缓存一致性方案

### 核心特性
- Lua 脚本保证原子性
- 版本号机制解决并发冲突
- 支持缓存重建和失效
- 防止脏数据

### 文件结构
```
internal/repository/cache/
├── rank_consistency.go                    # 一致性实现
└── lua/
    └── update_rank_with_version.lua       # Lua 脚本

examples/
└── rank_consistency_example.go            # 使用示例
```

### 一致性方案

#### 方案一：版本号 + CAS
```go
// 获取版本
version, _ := cache.GetVersion(ctx, userID, time.Now())

// 带版本更新
success, err := cache.IncrSignInScoreWithVersion(
    ctx, userID, time.Now(), 10, version,
)

if !success {
    // 版本冲突，重试
}
```

#### 方案二：缓存重建
```go
// 从数据库重建
cache.RebuildRankCache(ctx, time.Now(), dailyData, monthlyData)
```

#### 方案三：缓存失效
```go
// 主动失效
cache.InvalidateCache(ctx, time.Now())
```

### 并发测试
```go
// 10个 goroutine 并发更新同一用户
// 使用版本号保证最终一致性
```

---

## 🔐 5. 幂等键过期与补偿机制

### 核心特性
- 防止消息重复消费
- 支持 offset 和业务 ID 幂等
- 自动补偿过期的幂等键
- 定时任务扫描

### 文件结构
```
internal/repository/cache/
└── idempotent.go              # 幂等缓存

internal/worker/
└── idempotent_handler.go      # 幂等处理器

internal/service/
└── idempotent_compensate.go   # 补偿服务

cmd/compensate-job/
└── main.go                    # 补偿定时任务

examples/
└── idempotent_example.go      # 使用示例
```

### 幂等策略

#### 消息级别幂等
```go
// 基于 topic:partition:offset
handler := worker.NewIdempotentHandler(
    businessHandler,
    idempotentCache,
    worker.DefaultIdempotentKeyGenerator,
    24*time.Hour,
    logger,
)
```

#### 业务级别幂等
```go
// 基于业务 ID（订单号、用户ID等）
keyGen := worker.BusinessIdempotentKeyGenerator(func(msg *sarama.ConsumerMessage) string {
    return extractBusinessID(msg)
})

handler := worker.NewIdempotentHandler(
    businessHandler,
    idempotentCache,
    keyGen,
    24*time.Hour,
    logger,
)
```

### 补偿机制
```go
// 定期补偿过期的幂等键
compensateService := service.NewIdempotentCompensateService(
    idempotentCache,
    processRecordRepo,
    logger,
)

// 每小时执行一次
compensateService.CompensateExpiredKeys(ctx)
```

---

## 🚀 快速部署

### 1. 创建 Kafka Topic
使用提供的脚本快速创建所有需要的 topic：
```bash
bash script/create_kafka_topics.sh
```

或手动创建：
```bash
# 使用提供的脚本
bash script/create_kafka_topics.sh

# 或手动创建
kafka-topics.sh --create --topic user.registered \
  --bootstrap-server localhost:9092 \
  --partitions 1 --replication-factor 1

kafka-topics.sh --create --topic user.registered.dlq \
  --bootstrap-server localhost:9092 \
  --partitions 1 --replication-factor 1
```

### 2. 配置 Redis
```yaml
# config/dev.yaml
redis:
  addr: localhost:6379
  password: ""
  db: 1

ratelimit:
  enabled: true
  prefix: api-limiter
  interval: 1m
  limit: 100
```

### 3. 启动服务
```bash
# 主服务
go run cmd/notification-service/main.go

# Worker
go run cmd/worker/main.go

# 补偿任务
go run cmd/compensate-job/main.go
```

---

## 📊 监控指标

### 关键指标
- 死信队列消息数量
- 限流触发次数
- 幂等键补偿次数
- 缓存命中率
- 版本冲突次数

### 告警规则
- 死信队列消息堆积 > 100
- 限流触发率 > 10%
- 幂等补偿失败率 > 1%
- 缓存命中率 < 80%

---

## 🧪 测试

### 单元测试
```bash
# 运行所有测试
go test ./...

# 测试特定功能
go test ./internal/worker -v
go test ./internal/middleware -v
```

### 集成测试
```bash
# 启动依赖服务
docker-compose up -d

# 运行集成测试
go test ./internal/integration -v
```

---

## 📚 更多文档

- [详细功能说明](docs/features.md)
- [使用示例](examples/)
- [API 文档](docs/api.md)
- [性能优化指南](docs/performance.md)

---

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

## 📄 许可证

MIT License
