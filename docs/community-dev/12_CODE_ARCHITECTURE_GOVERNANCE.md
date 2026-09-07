# 12 代码架构治理：分层、模块化、接口、IoC 与横切能力

> 本文是 Coding Agent 的强制架构约束。优先遵循当前仓库已有结构，不为了“架构漂亮”进行全仓库重构。

## 1. 总原则

目标不是把目录拆得越多越好，而是让：

- 业务职责有清晰边界；
- 依赖方向单向；
- 基础设施可替换、可测试；
- 横切逻辑不污染业务代码；
- 新增模块能沿着固定调用链理解；
- 最终 Git diff 可以按层学习。

禁止出现“为了模块化而模块化”的目录爆炸。

---

## 2. 推荐调用链

现有项目继续坚持：

```text
HTTP
  ↓
web / handler
  ↓
service
  ↓
repository
  ↓
dao / storage / external infrastructure
```

异步链路：

```text
service
  ↓
outbox / domain event
  ↓
relay
  ↓
Kafka
  ↓
consumer handler
  ↓
service / repository
```

依赖装配：

```text
ioc / Wire
  ↓
construct concrete dependencies
  ↓
inject into handler / service / worker
```

### 依赖方向

允许：

```text
web          → service
service      → repository contract / events contract
repository   → dao / concrete storage adapter
integration  → external SDK / HTTP / Kafka / Redis client
ioc          → all concrete constructors
cmd          → ioc / application bootstrap
```

禁止：

```text
repository → web
dao        → service
domain     → Gin
domain     → GORM
domain     → Redis/Sarama SDK
service    → ioc
service    → global singleton
```

任何新增反向依赖必须先写 ADR 并由用户批准。

---

## 3. 目录治理

优先沿用仓库已有目录，而不是重建 Clean Architecture。

建议最终职责：

```text
cmd/
  user-center/
  worker/
  notification-service/

internal/
  domain/            # 纯业务实体、值对象、领域错误、事件定义（若现有项目已有）
  web/               # Gin handler、HTTP DTO、HTTP error mapping
  service/           # use-case / application orchestration
  repository/        # repository abstraction/concrete repo（遵循现有项目惯例）
  dao/               # GORM model、数据库细节
  integration/       # 外部系统 client，例如 ES
  events/            # event envelope / outbox / consumer contracts
  middleware/        # HTTP 横切能力（若现有目录已有则复用）
  config/            # 配置结构与加载

ioc/
  gin.go
  repository.go
  service.go
  kafka.go
  ...
```

### 禁止垃圾目录

未经批准不得新增：

```text
utils/
common/
helpers/
misc/
temp/
shared/
base/
core/
manager/
factory/
```

如果确实需要公共能力，目录名必须直接表达职责，例如：

```text
cursor/
clock/
idgen/
retry/
```

而不是万能 `utils`。

### 新目录准入条件

Agent 在 Phase A 必须回答：

1. 为什么现有 package 放不下？
2. 它的单一职责是什么？
3. 谁可以 import 它？
4. 它允许 import 谁？
5. 删除它会影响哪些模块？

答不清楚，不新增目录。

---

## 4. 模块化规则

Follow、Note、Feed、Ranking、Search 等是**业务模块**，不是每个都机械复制一套 10 层目录。

小模块可以共享现有：

```text
internal/web
internal/service
internal/repository
```

通过文件名区分：

```text
web/follow.go
service/follow.go
repository/follow.go
```

只有当一个模块明显膨胀、形成独立生命周期时，才考虑进一步拆 package。

### 禁止

```text
follow/
  handler/
  service/
  repository/
  dto/
  model/
  util/
```

对一个只有几百行的模块做这种 Java 式目录复制。

Go package 应保持简单、明确、低耦合。

---

## 5. 接口设计规则

### 什么时候应该有 interface

至少满足一项：

- service 依赖数据库/外部系统，需要测试替身；
- 同一能力确实存在多个实现；
- 明确的模块边界需要隔离基础设施；
- consumer 只需要依赖对方的一小部分能力。

### Go 风格

优先：

> **interface 定义在使用方附近，方法尽量少。**

例如 Feed Service 只需要：

```go
type FollowReader interface {
    ListFollowing(ctx context.Context, userID int64, cursor Cursor) (...)
}
```

不要因为某个 concrete repository 有 15 个方法，就把 15 个全部塞进接口。

### 禁止

- 每个 struct 都配一个 `IXXX` interface。
- 只有一个实现、没有测试边界却机械加接口。
- 巨型 `Repository` / `Service` interface。
- interface 返回 interface 的无意义抽象。
- 用 `any` 绕过类型系统。

### Constructor

默认：

```go
func NewNoteService(...) *NoteService
```

只有真正需要向消费者隐藏实现时才返回 interface。

---

## 6. IoC / Dependency Injection

本项目继续使用 **Wire 编译期注入**。

### 必须

- dependency 通过 constructor 注入；
- constructor 命名 `NewXxx` / 现有项目约定；
- Wire provider 只负责组装；
- `ioc/` 不承载业务逻辑；
- config 由上层读取后注入；
- HTTP client、Redis client、Kafka producer 等由装配层管理生命周期。

### 禁止

- Service Locator；
- 在 service 中调用全局 `GetRedis()` / `GetDB()`；
- package-level mutable singleton；
- 用 `init()` 建立 DB/Kafka/Redis 连接；
- handler 内部自行 new repository；
- repository 内部读取环境变量。

依赖关系应该可以从 constructor 参数直接看懂。

---

## 7. “AOP” 在 Go 中怎么做

本项目**不引入 Java 风格 AOP 框架**。

横切能力使用显式的：

### HTTP Middleware

适合：

- request ID
- access log
- panic recovery
- JWT authentication
- rate limit
- tracing（未来如果加）

调用：

```text
request
→ request-id middleware
→ logging middleware
→ auth/rate-limit
→ handler
```

禁止把具体 Feed/Note 业务判断塞进 middleware。

### Decorator / Wrapper

适合 service/repository/consumer 横切能力：

```text
LoggingRepository
MetricsRepository
CachedRepository
IdempotentConsumerHandler
RetryableConsumerHandler
```

形式：

```text
decorator
  ↓
inner interface
```

但只有真正有复用价值时才使用。

### Consumer Middleware / Handler Chain

Kafka 消费优先采用：

```text
decode
→ validation
→ idempotency
→ business handler
→ mark message
```

Retry、DLQ、logging 应围绕 handler 组合，而不是散落到每个业务 consumer。

---

## 8. Transaction 边界

事务应由**业务 use-case/service 层**决定。

例如发布 Note：

```text
service.PublishNote()
  ↓
BEGIN
  repository.CreateNote(tx)
  repository.CreateImages(tx)
  outbox.Append(tx)
COMMIT
```

Repository 不应偷偷创建一个外层不知道的独立事务，导致业务原子性失效。

跨 MySQL + Kafka：

- 不做“分布式事务假象”；
- 使用 MySQL transaction + Outbox；
- Kafka consumer 按 at-least-once + idempotency。

---

## 9. DTO / Domain / Persistence 分离

HTTP DTO 不要直接当 GORM Model 使用。

推荐：

```text
web request DTO
   ↓ map/validate
service input / domain
   ↓
repository
   ↓
DAO/GORM model
```

简单字段映射不需要引入 AutoMapper 类库，手写即可。

禁止让：

```text
gorm.Model
gin.Context
http.Request
```

穿透到 domain/service 核心逻辑。

---

## 10. Error 处理

### Service

返回可识别业务错误：

```text
ErrNotFound
ErrForbidden
ErrAlreadyFollowed
...
```

### Web

集中映射：

```text
business error
→ HTTP status + business code
```

禁止每个 service 自己生成 HTTP status。

### Infrastructure

底层错误：

```text
MySQL / Redis / Kafka / ES
```

必须 wrap 上下文，不能直接吞掉。

---

## 11. Context / Timeout

所有可能阻塞的 I/O：

```text
DB
Redis
Kafka
HTTP/ES
```

必须传播 `context.Context`。

禁止：

- 在深层重新创建 `context.Background()` 来绕过取消；
- 无限重试；
- 没有 timeout 的外部请求。

---

## 12. 配置

配置必须：

```text
config → ioc → constructor → component
```

禁止：

```text
service → os.Getenv()
repository → Viper.Get()
```

threshold、TTL、batch size、timeout 等工程参数要配置化，但也不要把所有常量都配置化。

---

## 13. 文件大小与职责

不使用死板的“每文件最多 N 行”规则。

但若出现以下信号，Agent 必须在 Phase A 提出拆分建议：

- 一个文件同时负责 HTTP、DB、Kafka；
- 一个 service 同时处理多个无关 use-case；
- 一个 package 出现大量 `xxx_helper.go`；
- 一个 constructor 依赖数量异常增长；
- interface 方法不断膨胀；
- import graph 开始形成循环倾向。

拆分要按职责，不按行数。

---

## 14. 架构变更必须留下证据

以下情况必须写 ADR：

- 引入新基础设施；
- 改变数据真相；
- 改变一致性模型；
- 改变 Feed 策略；
- 新增全局缓存；
- 新增跨模块 interface；
- 新增新的横切框架；
- 违反现有 dependency direction。

---

## 15. Agent 每个 Milestone 的架构自检

提交前回答：

1. 新代码的入口是什么？
2. 调用链是否仍然是单向的？
3. 是否新增了万能 package？
4. interface 是使用方真正需要的吗？
5. IoC 是否仍集中在 Wire/ioc？
6. 是否出现全局状态？
7. HTTP/GORM/Kafka SDK 是否泄漏到不该出现的层？
8. 横切逻辑应该是 middleware/decorator 还是业务逻辑？
9. 事务边界在哪里？
10. 这个模块以后如何独立测试？

任何一项解释不清楚，禁止进入 merge。
