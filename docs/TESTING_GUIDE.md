# 新功能测试指南

本文档提供了所有新增功能的详细测试步骤和方法。

---

## 📋 测试清单

- [ ] 1. 接口限流测试
- [ ] 2. Kafka 重试和死信队列测试
- [ ] 3. 幂等性测试
- [ ] 4. 热榜缓存一致性测试
- [ ] 5. 登录链路压测

---

## 🚦 1. 接口限流测试

### 前置条件
- Redis 已启动
- 服务已启动

### 测试步骤

#### 1.1 测试全局 IP 限流

```bash
# 快速发送 10 个请求，测试限流
for i in {1..10}; do
  curl -X POST http://localhost:8081/user/profile \
    -H "Content-Type: application/json" &
done
wait

# 预期结果：部分请求返回 429 Too Many Requests
```

#### 1.2 测试登录接口限流

```bash
# 同一 IP 连续登录 6 次（限制是 5 次/5分钟）
for i in {1..6}; do
  echo "第 $i 次尝试:"
  curl -X POST http://localhost:8081/user/login \
    -H "Content-Type: application/json" \
    -d '{"email":"test@example.com","password":"Test123!@#"}' \
    -w "\nHTTP Status: %{http_code}\n\n"
  sleep 1
done

# 预期结果：
# - 前 5 次：正常返回（200 或 401）
# - 第 6 次：返回 429，提示"登录尝试次数过多"
```

#### 1.3 测试账号级别限流

```bash
# 同一账号连续尝试 4 次（限制是 3 次/10分钟）
for i in {1..4}; do
  echo "第 $i 次尝试:"
  curl -X POST http://localhost:8081/user/login \
    -H "Content-Type: application/json" \
    -d '{"email":"specific@example.com","password":"WrongPassword"}' \
    -w "\nHTTP Status: %{http_code}\n\n"
  sleep 1
done

# 预期结果：第 4 次返回 429
```

#### 1.4 验证限流恢复

```bash
# 等待限流窗口过期后重试
echo "等待 5 分钟后重试..."
sleep 300

curl -X POST http://localhost:8081/user/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"Test123!@#"}'

# 预期结果：限流已恢复，可以正常请求
```

### 验证方法

**检查 Redis 中的限流键：**
```bash
redis-cli

# 查看所有限流键
KEYS *limiter*

# 查看某个键的详情
ZRANGE login:ratelimit:ip:127.0.0.1 0 -1 WITHSCORES

# 查看键的过期时间
TTL login:ratelimit:ip:127.0.0.1
```

---

## 🔄 2. Kafka 重试和死信队列测试

### 前置条件
- Kafka 已启动
- 创建必要的 topic

```bash
# 创建 topic
bash script/create_kafka_topics.sh

# 验证 topic 已创建
kafka-topics.sh --list --bootstrap-server localhost:9092
```

### 测试步骤

#### 2.1 模拟消息处理失败

**创建测试文件：** `test/kafka_retry_test.go`

```go
package test

import (
	"context"
	"encoding/json"
	"testing"
	"time"
	"user-center/internal/worker"
	"user-center/pkg/logger"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
)

func TestKafkaRetry(t *testing.T) {
	// 初始化
	cfg := sarama.NewConfig()
	cfg.Producer.Return.Successes = true
	producer, err := sarama.NewSyncProducer([]string{"localhost:9092"}, cfg)
	assert.NoError(t, err)
	defer producer.Close()

	l := logger.NewNoOpLogger()
	
	// 模拟失败的处理器（前 2 次失败，第 3 次成功）
	attemptCount := 0
	businessHandler := func(ctx context.Context, msg *sarama.ConsumerMessage) error {
		attemptCount++
		t.Logf("第 %d 次处理", attemptCount)
		
		if attemptCount < 3 {
			return errors.New("模拟处理失败")
		}
		return nil
	}

	retryHandler := worker.NewRetryableHandler(businessHandler, producer, l)

	// 创建测试消息
	msg := &sarama.ConsumerMessage{
		Topic:     "test.topic",
		Partition: 0,
		Offset:    1,
		Key:       []byte("test-key"),
		Value:     []byte(`{"user_id":123}`),
	}

	// 第 1 次处理（失败）
	err = retryHandler.Handle(context.Background(), msg)
	assert.Error(t, err)
	assert.Equal(t, 1, attemptCount)

	// 第 2 次处理（失败）
	err = retryHandler.Handle(context.Background(), msg)
	assert.Error(t, err)
	assert.Equal(t, 2, attemptCount)

	// 第 3 次处理（成功）
	err = retryHandler.Handle(context.Background(), msg)
	assert.NoError(t, err)
	assert.Equal(t, 3, attemptCount)
}
```

#### 2.2 测试死信队列

**手动发送消息到 Kafka：**

```bash
# 发送测试消息
echo '{"user_id":123,"email":"test@example.com"}' | \
  kafka-console-producer.sh \
    --broker-list localhost:9092 \
    --topic user.registered
```

**修改 worker 代码模拟失败：**

在 `internal/worker/user_registered_handler.go` 中临时添加：
```go
func (h *UserRegisteredHandler) Handle(ctx context.Context, msg *sarama.ConsumerMessage) error {
    // 临时添加：模拟处理失败
    return errors.New("测试失败")
}
```

**启动 worker 并观察日志：**
```bash
go run cmd/worker/main.go

# 观察日志输出：
# - 第 1 次失败 -> 1秒后重试
# - 第 2 次失败 -> 5秒后重试
# - 第 3 次失败 -> 30秒后重试
# - 第 4 次失败 -> 发送到 DLQ
```

**检查死信队列：**
```bash
# 消费死信队列消息
kafka-console-consumer.sh \
  --bootstrap-server localhost:9092 \
  --topic user.registered.dlq \
  --from-beginning

# 预期输出：包含原始消息和重试元数据
```

---

## 🔐 3. 幂等性测试

### 前置条件
- Redis 已启动

### 测试步骤

#### 3.1 单元测试

**创建测试文件：** `internal/worker/idempotent_handler_test.go`

```go
package worker

import (
	"context"
	"testing"
	"time"
	"user-center/internal/repository/cache"
	"user-center/pkg/logger"

	"github.com/IBM/sarama"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestIdempotentHandler(t *testing.T) {
	// 使用 miniredis 模拟 Redis
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	idempotentCache := cache.NewRedisIdempotentCache(client, "test")
	l := logger.NewNoOpLogger()

	// 记录处理次数
	processCount := 0
	businessHandler := func(ctx context.Context, msg *sarama.ConsumerMessage) error {
		processCount++
		return nil
	}

	handler := NewIdempotentHandler(
		businessHandler,
		idempotentCache,
		DefaultIdempotentKeyGenerator,
		time.Hour,
		l,
	)

	msg := &sarama.ConsumerMessage{
		Topic:     "test.topic",
		Partition: 0,
		Offset:    100,
	}

	// 第 1 次处理
	err = handler.Handle(context.Background(), msg)
	assert.NoError(t, err)
	assert.Equal(t, 1, processCount)

	// 第 2 次处理（应该被跳过）
	err = handler.Handle(context.Background(), msg)
	assert.NoError(t, err)
	assert.Equal(t, 1, processCount, "消息不应该被重复处理")

	// 第 3 次处理（应该被跳过）
	err = handler.Handle(context.Background(), msg)
	assert.NoError(t, err)
	assert.Equal(t, 1, processCount, "消息不应该被重复处理")
}
```

#### 3.2 集成测试

**手动测试：**

```bash
# 1. 启动 worker
go run cmd/worker/main.go

# 2. 发送相同的消息 3 次
for i in {1..3}; do
  echo '{"user_id":999,"email":"idempotent@test.com"}' | \
    kafka-console-producer.sh \
      --broker-list localhost:9092 \
      --topic user.registered
  sleep 1
done

# 3. 检查日志，应该只处理 1 次
# 预期日志：
# - 第 1 次：正常处理
# - 第 2 次：消息已处理，跳过
# - 第 3 次：消息已处理，跳过
```

**验证 Redis 中的幂等键：**
```bash
redis-cli

# 查看幂等键
KEYS idempotent:*

# 查看键的值和 TTL
GET idempotent:user.registered:0:100
TTL idempotent:user.registered:0:100
```

---

## 🔥 4. 热榜缓存一致性测试

### 前置条件
- Redis 已启动

### 测试步骤

#### 4.1 单元测试

**创建测试文件：** `internal/repository/cache/rank_consistency_test.go`

```go
package cache

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestRankConsistency(t *testing.T) {
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	cache := NewRankConsistencyCache(client)
	
	err = cache.Init(context.Background())
	assert.NoError(t, err)

	ctx := context.Background()
	userID := int64(123)
	now := time.Now()

	// 测试版本号更新
	version, err := cache.GetVersion(ctx, userID, now)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), version)

	// 第 1 次更新
	success, err := cache.IncrSignInScoreWithVersion(ctx, userID, now, 10, version)
	assert.NoError(t, err)
	assert.True(t, success)

	// 版本号应该增加
	newVersion, err := cache.GetVersion(ctx, userID, now)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), newVersion)

	// 使用旧版本号更新（应该失败）
	success, err = cache.IncrSignInScoreWithVersion(ctx, userID, now, 10, version)
	assert.NoError(t, err)
	assert.False(t, success, "使用旧版本号应该失败")
}

func TestConcurrentUpdate(t *testing.T) {
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	cache := NewRankConsistencyCache(client)
	cache.Init(context.Background())

	ctx := context.Background()
	userID := int64(456)
	now := time.Now()

	// 10 个 goroutine 并发更新
	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for retry := 0; retry < 5; retry++ {
				version, _ := cache.GetVersion(ctx, userID, now)
				success, err := cache.IncrSignInScoreWithVersion(ctx, userID, now, 1, version)
				
				if err == nil && success {
					mu.Lock()
					successCount++
					mu.Unlock()
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
		}()
	}

	wg.Wait()

	// 所有更新都应该成功
	assert.Equal(t, 10, successCount)

	// 最终版本号应该是 10
	finalVersion, _ := cache.GetVersion(ctx, userID, now)
	assert.Equal(t, int64(10), finalVersion)
}
```

#### 4.2 手动测试并发更新

**创建测试脚本：** `test/concurrent_rank_update.go`

```go
package main

import (
	"context"
	"fmt"
	"sync"
	"time"
	"user-center/internal/repository/cache"

	"github.com/redis/go-redis/v9"
)

func main() {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	rankCache := cache.NewRankConsistencyCache(client)
	if err := rankCache.Init(context.Background()); err != nil {
		panic(err)
	}

	ctx := context.Background()
	userID := int64(12345)
	now := time.Now()

	fmt.Println("开始并发更新测试...")
	fmt.Printf("用户 ID: %d\n", userID)
	fmt.Println("并发数: 20")
	fmt.Println("每个 goroutine 更新 1 分")
	fmt.Println("预期最终版本号: 20")
	fmt.Println()

	var wg sync.WaitGroup
	successCount := 0
	failCount := 0
	var mu sync.Mutex

	startTime := time.Now()

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			for retry := 0; retry < 10; retry++ {
				version, err := rankCache.GetVersion(ctx, userID, now)
				if err != nil {
					fmt.Printf("[G%02d] 获取版本失败: %v\n", index, err)
					continue
				}

				success, err := rankCache.IncrSignInScoreWithVersion(ctx, userID, now, 1, version)
				if err != nil {
					fmt.Printf("[G%02d] 更新失败: %v\n", index, err)
					continue
				}

				if success {
					mu.Lock()
					successCount++
					mu.Unlock()
					fmt.Printf("[G%02d] ✓ 更新成功 (重试 %d 次)\n", index, retry)
					return
				}

				// 版本冲突，重试
				fmt.Printf("[G%02d] ✗ 版本冲突，重试 %d\n", index, retry+1)
				time.Sleep(10 * time.Millisecond)
			}

			mu.Lock()
			failCount++
			mu.Unlock()
			fmt.Printf("[G%02d] ✗ 重试次数耗尽\n", index)
		}(i)
	}

	wg.Wait()
	duration := time.Since(startTime)

	fmt.Println()
	fmt.Println("========== 测试结果 ==========")
	fmt.Printf("总耗时: %v\n", duration)
	fmt.Printf("成功: %d\n", successCount)
	fmt.Printf("失败: %d\n", failCount)

	finalVersion, _ := rankCache.GetVersion(ctx, userID, now)
	fmt.Printf("最终版本号: %d (预期: 20)\n", finalVersion)

	if finalVersion == 20 && successCount == 20 {
		fmt.Println("✓ 测试通过！")
	} else {
		fmt.Println("✗ 测试失败！")
	}
}
```

**运行测试：**
```bash
go run test/concurrent_rank_update.go
```

---

## ⚡ 5. 登录链路压测

### 前置条件
- 服务已启动
- 已创建测试账号

### 测试步骤

#### 5.1 准备测试数据

```bash
# 创建测试账号
for i in {1..5}; do
  curl -X POST http://localhost:8081/user/signup \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"test${i}@example.com\",\"password\":\"Test123!@#\",\"confirmed_password\":\"Test123!@#\"}"
done
```

#### 5.2 使用 wrk 压测

```bash
# 基础压测（12 线程，400 连接，持续 30 秒）
wrk -t12 -c400 -d30s --latency \
  -s script/benchmark_login.lua \
  http://localhost:8081/user/login

# 或使用提供的脚本
bash script/benchmark_run.sh
```

#### 5.3 使用 ab 压测

```bash
# 创建请求体文件
cat > login.json << EOF
{"email":"test1@example.com","password":"Test123!@#"}
EOF

# 执行压测
ab -n 10000 -c 100 \
  -p login.json \
  -T application/json \
  http://localhost:8081/user/login
```

#### 5.4 分析压测结果

**关键指标：**
- QPS (每秒请求数)
- 平均延迟
- P50/P90/P99 延迟
- 错误率

**优化目标：**
- QPS > 1000
- P99 延迟 < 100ms
- 错误率 < 0.1%

---

## 🧪 完整测试流程

### 一键测试脚本

**创建：** `test/run_all_tests.sh`

```bash
#!/bin/bash

echo "=========================================="
echo "开始执行所有功能测试"
echo "=========================================="
echo ""

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# 测试结果统计
PASSED=0
FAILED=0

# 1. 单元测试
echo "1. 运行单元测试..."
if go test ./internal/worker -v; then
    echo -e "${GREEN}✓ Worker 单元测试通过${NC}"
    ((PASSED++))
else
    echo -e "${RED}✗ Worker 单元测试失败${NC}"
    ((FAILED++))
fi
echo ""

if go test ./internal/repository/cache -v; then
    echo -e "${GREEN}✓ Cache 单元测试通过${NC}"
    ((PASSED++))
else
    echo -e "${RED}✗ Cache 单元测试失败${NC}"
    ((FAILED++))
fi
echo ""

# 2. 限流测试
echo "2. 测试接口限流..."
echo "发送 10 个并发请求..."
for i in {1..10}; do
    curl -s -X GET http://localhost:8081/rank/daily > /dev/null &
done
wait
echo -e "${GREEN}✓ 限流测试完成（请检查日志）${NC}"
echo ""

# 3. Kafka 测试
echo "3. 测试 Kafka 消息..."
echo '{"user_id":999,"email":"test@example.com"}' | \
  kafka-console-producer.sh \
    --broker-list localhost:9092 \
    --topic user.registered 2>/dev/null
echo -e "${GREEN}✓ Kafka 消息已发送${NC}"
echo ""

# 4. 幂等性测试
echo "4. 测试幂等性..."
if [ -f "test/concurrent_rank_update.go" ]; then
    if go run test/concurrent_rank_update.go; then
        echo -e "${GREEN}✓ 幂等性测试通过${NC}"
        ((PASSED++))
    else
        echo -e "${RED}✗ 幂等性测试失败${NC}"
        ((FAILED++))
    fi
else
    echo "跳过（测试文件不存在）"
fi
echo ""

# 总结
echo "=========================================="
echo "测试完成"
echo "=========================================="
echo -e "通过: ${GREEN}${PASSED}${NC}"
echo -e "失败: ${RED}${FAILED}${NC}"
echo ""

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}所有测试通过！${NC}"
    exit 0
else
    echo -e "${RED}部分测试失败，请检查日志${NC}"
    exit 1
fi
```

**运行：**
```bash
chmod +x test/run_all_tests.sh
bash test/run_all_tests.sh
```

---

## 📊 监控和验证

### Redis 监控

```bash
# 实时监控 Redis 命令
redis-cli monitor

# 查看内存使用
redis-cli info memory

# 查看所有键
redis-cli --scan --pattern '*'
```

### Kafka 监控

```bash
# 查看消费者组状态
kafka-consumer-groups.sh \
  --bootstrap-server localhost:9092 \
  --describe \
  --group user-center-worker

# 查看 topic 消息数量
kafka-run-class.sh kafka.tools.GetOffsetShell \
  --broker-list localhost:9092 \
  --topic user.registered
```

### 应用日志

```bash
# 实时查看日志
tail -f logs/user-center.log

# 过滤限流日志
grep "限流" logs/user-center.log

# 过滤重试日志
grep "重试" logs/user-center.log

# 过滤幂等日志
grep "幂等" logs/user-center.log
```

---

## ✅ 测试检查清单

完成所有测试后，确认以下内容：

- [ ] 限流功能正常工作，超过限制返回 429
- [ ] Kafka 消息失败后自动重试
- [ ] 重试 3 次后消息进入死信队列
- [ ] 相同消息不会被重复处理（幂等性）
- [ ] 并发更新热榜时数据一致
- [ ] 登录接口 QPS 达到预期
- [ ] P99 延迟在可接受范围内
- [ ] 无内存泄漏或资源泄漏
- [ ] Redis 键正确设置了过期时间
- [ ] 日志输出清晰，便于排查问题

---

## 🐛 常见问题

### 1. 限流不生效
- 检查 Redis 连接是否正常
- 确认配置文件中 `ratelimit.enabled: true`
- 查看 Redis 中是否有限流键

### 2. Kafka 消息不重试
- 确认 Producer 已正确初始化
- 检查死信队列 topic 是否已创建
- 查看 worker 日志是否有错误

### 3. 幂等键过期
- 检查 TTL 设置是否合理
- 确认补偿任务是否正常运行
- 查看 Redis 内存是否充足

### 4. 压测结果不理想
- 检查数据库连接池配置
- 确认 Redis 性能是否达标
- 查看是否有慢查询
- 检查网络延迟

---

## 📚 参考资料

- [wrk 使用文档](https://github.com/wg/wrk)
- [Apache Bench 文档](https://httpd.apache.org/docs/2.4/programs/ab.html)
- [Kafka 测试工具](https://kafka.apache.org/documentation/#quickstart)
- [Redis 性能测试](https://redis.io/topics/benchmarks)
