# AGENTS.md

## 1. 项目使命

你正在维护现有 `User-center` Go 后端。目标是**增量演进**为生活图文社区后端，而不是重写项目。

必须优先复用：Go、Gin、GORM、MySQL、Redis、Kafka/Sarama、Wire、Viper、Zap、Docker Compose、Handler/Service/Repository/DAO 分层、`internal/events`、MySQL Outbox+Relay、Kafka Consumer Group、Redis Lua Consumer Idempotency、worker、notification-service。

## 2. 禁止事项

未经明确批准，Agent 不得：

- 把 Kafka 换成 RabbitMQ。
- 引入 Kubernetes、etcd、gRPC、服务注册中心、分布式事务框架。
- 大规模移动目录或重写现有模块。
- 一次实现多个 milestone。
- 改动与当前 milestone 无关的文件。
- 删除现有测试或降低断言强度来让 CI 通过。
- 使用 `git reset --hard`、`git clean -fd`、force push、修改历史提交。
- 写入或提交密钥、token、密码。
- 编造 QPS、P95/P99、缓存命中率等性能数据。
- 为了“简历好看”引入没有业务必要的新组件。

## 3. 每次开发固定协议

### Phase A：只读审计

接到 milestone 后，先阅读 `AGENTS.md`、对应 milestone 文档和相关现有代码，输出：

1. 当前调用链。
2. 计划修改文件。
3. 新增文件。
4. DB/Redis/Kafka 变化。
5. 并发与一致性风险。
6. 测试方案。
7. 不确定项。

**此阶段禁止改代码，等待用户批准。**

### Phase B：小步实现

获得批准后：

- 只实现当前 milestone。
- 保持每次改动可编译。
- 优先复用现有抽象。
- 新依赖必须说明必要性。
- 每完成一个逻辑单元先运行相关测试。

### Phase C：验证

最低要求：

```bash
gofmt -w <changed go files>
go test ./...
go vet ./...
git diff --check
```

涉及并发、缓存、worker、Kafka 时：

```bash
go test -race ./...
```

### Phase D：交接

完成后输出：修改文件、核心调用链、表/索引、Redis key、Kafka event/topic/group、失败场景、测试命令、未解决问题、3 个用户自行编码的小任务，并更新 Learning Handoff。

## 4. Git 规则

每个 milestone 独立分支：

```text
feat/community-m01-follow
feat/community-m02-note
feat/community-m03-engagement
feat/community-m04-pull-feed
feat/community-m05-push-feed
feat/community-m06-hybrid-feed
feat/community-m07-note-cache
feat/community-m08-hot-ranking
feat/community-m09-search
feat/community-m10-notification
chore/community-m11-reliability
```

一个 milestone 推荐 3~6 个可理解提交：

```text
docs(m04): record pull feed design
feat(m04): add feed repository and cursor
feat(m04): add feed service and handler
test(m04): cover stable cursor pagination
docs(m04): add learning handoff
```

提交前必须看：

```bash
git status --short
git diff --stat
git diff
git diff --check
go test ./...
```

## 5. 代码设计规则

- 正确性优先，优化必须有 baseline。
- 列表默认稳定排序；动态 Feed 优先 cursor pagination。
- 业务幂等优先由数据库约束/唯一键保证，不只依赖应用层 `if`。
- Outbox 解决 DB 与消息发布的一致性，但不保证 exactly-once。
- 消费者按 at-least-once 设计，必须考虑重复消费。
- Redis 是加速层，不应成为核心业务唯一真相，除非文档明确说明。
- 降级不能把主库拖死。
- 缓存一致性必须写明允许的旧数据窗口。
- 超时、重试必须有边界，不允许无限重试。
- 关键日志携带 `request_id`、`event_id`、`user_id`、`note_id` 等关联字段。

## 6. Agent 输出要求

不要只说“已完成”。必须解释为什么这样改、为什么不选另一种方案、trade-off、如何证明正确、如何证明优化有效。

## 7. 架构治理（强制）

每次 Phase A 必须额外阅读：

- `docs/community-dev/12_CODE_ARCHITECTURE_GOVERNANCE.md`
- `docs/community-dev/13_DIRECTORY_BLUEPRINT.md`

新增代码必须遵循：

```text
web/handler
→ service
→ repository
→ dao/integration
```

异步链路遵循：

```text
service → outbox → Kafka → consumer handler → service/repository
```

强制要求：

- IoC 集中在 `ioc/` + Wire；业务层不得使用 Service Locator 或全局依赖。
- interface 只用于真实模块边界/测试替身/多实现，不得为每个 struct 机械创建接口。
- Go 中不引入 Java 式 AOP；HTTP 使用 middleware，service/repository/consumer 横切能力使用 decorator/wrapper。
- 禁止新增 `utils/common/helpers/misc/shared/base/core` 等万能 package，除非用户书面批准。
- HTTP DTO、domain/service 对象、GORM persistence model 不得无边界混用。
- Gin/GORM/Redis/Sarama/Elasticsearch SDK 不得向不属于基础设施的层泄漏。
- transaction boundary 必须在 use-case/service 层可理解。
- 新目录、新跨模块 interface、新横切机制必须在 Phase A 说明依赖方向和必要性。
- PR 合并前必须执行 `templates/ARCHITECTURE_REVIEW_CHECKLIST.md`。
