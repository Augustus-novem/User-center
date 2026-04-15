# 新增功能说明

## 1. Kafka 消费失败重试 + 死信队列

### 功能说明
- 消息处理失败时自动重试，最多重试 3 次
- 采用指数退避策略：1秒 -> 5秒 -> 30秒
- 超过最大重试次数后自动发送到死信队列（DLQ）
- 记录完整的重试元数据（原始 topic、partition、offset、重试次数、错误信息等）

### 使用方式
```go
// 在 worker 中使用
handler := worker.NewRetryableHandler(
    yourBusinessHandler,
    kafkaProducer,
    logger,
)

// 处理消息
err := handler.Handle(ctx, msg)
```

### 死信队列命名规则
- 原 topic: `user.registered`
- 死信 topic: `user.registered.dlq`

### 相关文件
- `internal/worker/retry_handler.go` - 重试处理器实现

---

## 2. 接口限流

### 功能说明
- 基于 Redis 的滑动窗口限流算法
- 支持 IP 级别和用户级别限流
- 登录接口专用限流器，防止暴力破解

### 限流策略

#### 全局限流
- 同一 IP：1分钟内最多 100 次请求
- 配置文件：`config/dev.yaml` 中的 `ratelimit` 配置

#### 登录接口限流
- 同一 IP：5分钟内最多 5 次登录尝试
- 同一账号：10分钟内最多 3 次登录尝试
- 响应头返回剩余次数：`X-RateLimit-Remaining`

### 使用方式
```go
// 全局限流
limiter := middleware.NewRateLimiter(redisClient, "api-limit", time.Minute, 100)
router.Use(limiter.Build())

// 用户级别限流
router.Use(limiter.BuildWithUserID())

// 登录限流
loginLimiter := middleware.NewLoginRateLimiter(redisClient)
router.POST("/login", loginLimiter.Build(), loginHandler)
```

### 相关文件
- `internal/web/middleware/ratelimit.go` - 通用限流中间件
- `internal/web/middleware/login_ratelimit.go` - 登录专用限流

---

## 3. 登录链路压测与优化

### 优化建议

#### 数据库优化
1. 为 `email` 和 `phone` 字段添加索引
2. 使用连接池，配置合理的最大连接数
3. 读写分离：登录查询走从库

#### 缓存优化
1. 用户信息缓存（减少数据库查询）
2. JWT Token 缓存验证
3. 使用 Redis Pipeline 批量操作

#### 限流保护
1. 接口级别限流（已实现）
2. 用户级别限流（已实现）
3. 验证码限流

#### 性能监控
1. 添加 Prometheus 指标
2. 记录慢查询日志
3. 监控 Redis 命中率

### 压测命令示例
```bash
# 使用 wrk 进行压测
wrk -t12 -c400 -d30s --latency \
  -s script/benchmark_login.lua \
  http://localhost:8081/user/login

# 或使用提供的脚本
bash script/benchmark_run.sh

# 使用 ab 进行压测
ab -n 10000 -c 100 \
  -p login.json -T application/json \
  http://localhost:8081/user/login
```

---

## 4. 热榜缓存一致性方案

### 功能说明
- 使用 Lua 脚本保证原子性操作
- 版本号机制解决并发更新问题
- 支持缓存重建和失效策略

### 一致性保证

#### 方案一：版本号 + Lua 脚本（已实现）
- 每次更新前检查版本号
- 使用 Lua 脚本保证原子性
- 版本冲突时返回失败，由业务层重试

#### 方案二：缓存重建
- 定期从数据库重建缓存
- 适用于数据不一致时的修复

#### 方案三：缓存失效
- 数据变更时主动失效缓存
- 下次查询时重新加载

### 使用方式
```go
// 带版本号的更新
cache := cache.NewRankConsistencyCache(redisClient)
cache.Init(ctx) // 初始化 Lua 脚本

// 获取当前版本
version, _ := cache.GetVersion(ctx, userID, time.Now())

// 更新积分（带版本检查）
success, err := cache.IncrSignInScoreWithVersion(ctx, userID, time.Now(), 10, version)
if !success {
    // 版本冲突，重试
}

// 重建缓存
cache.RebuildRankCache(ctx, time.Now(), dailyData, monthlyData)

// 失效缓存
cache.InvalidateCache(ctx, time.Now())
```

### 相关文件
- `internal/repository/cache/rank_consistency.go` - 一致性实现
- `internal/repository/cache/lua/update_rank_with_version.lua` - Lua 脚本

---

## 5. 幂等键过期与补偿机制

### 功能说明
- 防止消息重复消费
- 幂等键过期后自动补偿
- 支持基于 offset 和业务 ID 的幂等

### 幂等策略

#### 消息级别幂等
- 基于 `topic:partition:offset` 生成幂等键
- 适用于保证消息不重复处理

#### 业务级别幂等
- 基于业务 ID（如订单号、用户ID）生成幂等键
- 适用于业务去重

### 补偿机制
- 定期扫描处理成功的记录
- 检查幂等键是否过期
- 过期则重新设置，防止重复处理

### 使用方式
```go
// 消息幂等处理
idempotentCache := cache.NewRedisIdempotentCache(redisClient, "idempotent")
handler := worker.NewIdempotentHandler(
    businessHandler,
    idempotentCache,
    worker.DefaultIdempotentKeyGenerator, // 或自定义生成器
    24*time.Hour, // TTL
    logger,
)

// 业务 ID 幂等
businessKeyGen := worker.BusinessIdempotentKeyGenerator(func(msg *sarama.ConsumerMessage) string {
    // 从消息中提取业务 ID
    return extractBusinessID(msg)
})

// 补偿服务
compensateService := service.NewIdempotentCompensateService(
    idempotentCache,
    processRecordRepo,
    logger,
)

// 定期执行补偿
compensateService.CompensateExpiredKeys(ctx)
```

### 相关文件
- `internal/repository/cache/idempotent.go` - 幂等缓存
- `internal/worker/idempotent_handler.go` - 幂等处理器
- `internal/service/idempotent_compensate.go` - 补偿服务

---

## 配置示例

### config/dev.yaml 新增配置
```yaml
# 限流配置
ratelimit:
  enabled: true
  prefix: api-limiter
  interval: 1m
  limit: 100

# Kafka 配置（已有，确保启用）
kafka:
  enabled: true
  brokers:
    - localhost:9092
  client_id: user-center
  consumer_group: user-center-worker
```

---

## 部署建议

### 1. Kafka Topic 创建
```bash
# 创建死信队列 topic
kafka-topics.sh --create \
  --topic user.registered.dlq \
  --bootstrap-server localhost:9092 \
  --partitions 1 \
  --replication-factor 1

kafka-topics.sh --create \
  --topic user.activity.dlq \
  --bootstrap-server localhost:9092 \
  --partitions 1 \
  --replication-factor 1
```

### 2. Redis 配置
- 确保 Redis 版本 >= 6.0
- 配置合理的内存淘汰策略：`maxmemory-policy allkeys-lru`
- 开启持久化（AOF 或 RDB）

### 3. 监控告警
- 监控死信队列消息数量
- 监控限流触发次数
- 监控幂等键补偿次数
- 监控缓存命中率

---

## 测试建议

### 1. 重试机制测试
```go
// 模拟处理失败
func TestRetryHandler(t *testing.T) {
    // 第1次失败 -> 1秒后重试
    // 第2次失败 -> 5秒后重试
    // 第3次失败 -> 30秒后重试
    // 第4次失败 -> 发送到 DLQ
}
```

### 2. 限流测试
```bash
# 并发测试
for i in {1..10}; do
  curl -X POST http://localhost:8081/user/login \
    -H "Content-Type: application/json" \
    -d '{"email":"test@example.com","password":"Test123!@#"}' &
done
```

### 3. 幂等测试
```go
// 重复发送相同消息
func TestIdempotent(t *testing.T) {
    // 第1次处理成功
    // 第2次应该被跳过
}
```

### 4. 缓存一致性测试
```go
// 并发更新测试
func TestRankConsistency(t *testing.T) {
    // 多个 goroutine 同时更新同一用户积分
    // 验证最终结果正确
}
```
