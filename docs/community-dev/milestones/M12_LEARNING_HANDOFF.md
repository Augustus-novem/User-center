# M12 Learning Handoff

## 1. 核心调用链

```text
HTTP → Gin middleware/body cap → handler → service → repository → DAO/cache

service transaction → MySQL business row + outbox
  → relay → Kafka source topic
  → consumer group → permanent classification → synchronous .dlq → mark source
                   ↘ transient/BUSY → no mark → Kafka redelivery
                   ↘ ACQUIRED → business effect → owner-checked MarkDone → mark source
```

Search rebuild：

```text
MySQL published notes → temporary physical ES index
  → all writes succeed → atomic _aliases swap → delete old physical index
  ↘ failure → bounded abort/delete temp → old alias remains available
```

Hybrid Feed：

```text
Inbox cursor scan → hydrate/filter until enough or exhausted
Celebrity follow cursor scan → published notes
→ one final dedup + note_id DESC + limit+1
```

## 2. 关键实现与理由

- `worker.RunDeduplicated` 集中三态与 owner-token 状态机，避免每个 handler 重新实现且出现语义漂移。
- cleanup 使用 `context.WithoutCancel` 加短 timeout：Kafka session 取消不能跳过 lease 清理，但 cleanup 也不能无限阻塞。
- poison classification 只区分永久输入错误与瞬时基础设施错误；没有接入 timestamp retry，因为 Kafka producer timestamp 不提供延迟投递。
- Feed 两路独立收集再 merge，避免 celebrity 候选过早填满页而永久遮蔽后续有效 Inbox。
- ES alias swap 把 rebuild 从原地覆盖变成可回滚发布；代价是 rebuild 期间需要一份额外索引空间。
- JWT idle key 可滑动、absolute key 固定过期；刷新 token 携带 absolute deadline 且 Lua 用 absolute key PTTL 封顶，刷新不能延长绝对生命周期。

## 3. 数据与基础设施变化

### MySQL

没有 schema、表或索引变化。MySQL 仍是用户、Note、关系、互动、通知与 Outbox 的真相源。

### Redis

- `consumer:event:processing:{namespace}:{event_id}`：随机 owner token，默认 5 分钟 lease。
- `consumer:event:done:{namespace}:{event_id}`：完成标记，默认 7 天。
- `user:refresh:ssid:{ssid}`：refresh JTI，同时作为 sliding idle lease。
- `user:session:absolute:ssid:{ssid}`：固定 absolute deadline/TTL。
- `user:ssid:{ssid}`：access session/logout marker；touch TTL 不超过 absolute PTTL。
- `user:info:{user_id}`：只序列化 profile DTO，不含 password hash。

### Kafka

- 源 topic/group 保持 `15_EVENT_CATALOG.md` 定义不变。
- 每个源 topic 增加 `${topic}.dlq` 投递路径；消息保留原 key/value/header，并附带原 topic/partition/offset/error header。
- DLQ publish 失败、瞬时错误或 `BUSY` 都不 mark source message。

### Elasticsearch

- 配置的 `search.index` 作为 alias。
- physical index 命名为 `{alias}-rebuild-{uuid}`；只在 alias 原子切换后删除旧 physical index。

## 4. 失败场景与边界

- stale owner 清理/完成新 lease：Lua 返回 ownership lost，不改变 Redis key。
- 业务部分成功：清理本 lease，重投依赖下游 DB unique、ZADD 或固定 ES document ID 保持幂等。
- `MarkDone` 或 DLQ publish 失败：handler 返回错误，Kafka source offset 不提交。
- 搜索重建中途失败：删除 temp，旧 alias 可读；若 cleanup 也失败，两个错误都会返回，便于人工清理孤儿索引。
- Redis auth backend 不可用：HTTP 返回服务不可用/服务端错误，不伪装成用户 token 无效。
- release LocalSMS 与 placeholder secret 会在启动验证阶段被拒绝。
- 没有新增 retry topic、DLQ consumer、Outbox 多实例 claim/lease、生产 HA 或完整安全平台。

## 5. Architecture review

- [x] 保持 web → service → repository → dao/cache。
- [x] 异步依赖保持 service → outbox → Kafka → consumer handler → service/repository。
- [x] Gin、Redis、Sarama、Elasticsearch SDK 没有向非基础设施层新增泄漏。
- [x] 没有新增万能 package、Service Locator、全局业务依赖或无业务必要的 interface。
- [x] 没有新增数据库、消息系统、中间件或部署平台。
- [x] 删除项均为确认未接线的实验/失效文档，不改变运行时行为。

## 6. Verification

- R1 targeted Kafka poison-message + Elasticsearch alias E2E：PASS。
- `go build ./...`：PASS。
- `go test ./...`：PASS。
- `go vet ./...`：PASS。
- `govulncheck ./...`：PASS（无 reachable vulnerability）。
- `git diff --check`：PASS。
- `go test -race ./...`：由 Linux GitHub Actions 执行；按约束不在 Windows 安装 GCC/MSYS2/Clang/Zig。

## 7. 我自己编码的三个任务

1. 为 deduplicator 写一个 lease 到期后新 owner 获取、旧 owner 同时尝试 `MarkDone` 的并发测试，并解释线性化点。
2. 为 Hybrid Feed 增加多页 cursor 场景，手工证明 push/pull 去重后不会跨页重复。
3. 为 Search rebuild 增加孤儿临时索引巡检设计，只写接口、触发条件和告警字段，不实现新的定时平台。

> Agent 不提供本节答案，除非我完成尝试后请求 hint。
