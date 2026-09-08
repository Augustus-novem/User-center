# M12 Community v1.1 有限修复

## 目标与边界

在保留 `community-v1` 原始标签的前提下，只修复已由代码 review 确定的 R1 correctness、R2 security hardening、R3 dependency/CI 问题，并做行为不变的 Webook 对齐清理。没有新增业务、中间件、数据库、消息系统或部署平台。

## R1 Correctness

### Kafka consumer

- Redis deduplicator 明确返回 `ACQUIRED`、`DONE`、`BUSY`；processing key 保存 `crypto/rand` owner token。
- `MarkDone` 与 `ClearInFlight` 通过 Lua 校验 owner，旧 lease 不能结束或清理新 lease。
- `DONE` 可提交；`BUSY` 返回 `ErrMessageInFlight` 且不提交；业务成功后只有 `MarkDone` 成功才提交。
- 业务失败后，用脱离 Kafka session cancellation、2 秒有界的 context 清理本 owner lease。
- 永久非法 JSON/event 同步发布到 `${topic}.dlq`，只有 DLQ ack 后才提交源消息；Redis/MySQL/ES/timeout 等瞬时错误保留源 offset 等待重投。
- 删除以 `ProducerMessage.Timestamp` 假装 broker 延迟的未接线 retry 实验。

### Hybrid Feed

- Inbox 独立游标扫描并 hydration/filter，直到获得 `limit+1` 个有效 push candidates 或 Inbox 自然结束。
- Celebrity follows 独立游标扫描到 exhaustion，不再有 1000 follows 隐藏上限。
- 两路候选只在最后执行一次 dedup、`note_id DESC` 和 `limit+1` 裁剪。

### Search rebuild

- `search.index` 是运行时 alias；Index/Delete/Search 始终访问 alias。
- Rebuild 创建临时 physical index，写入全部 MySQL published notes，成功后通过 `_aliases` 原子切换，再删除旧 physical index。
- 写入或切换失败时，以独立 5 秒 cleanup context 删除临时索引并保持旧 alias；cleanup 失败会合并到返回错误中。

## R2 Security hardening

- `net/http.Server` 设置 header/read/write/idle timeout，并在 SIGINT/SIGTERM 后执行有界 graceful shutdown。
- Gin 默认 `SetTrustedProxies(nil)`，请求体有统一大小上限；WeChat HTTP client 有 timeout。
- OTP 使用 `crypto/rand`；LocalSMS 不记录验证码或完整手机号；release 默认且强制关闭 LocalSMS 登录。
- Redis user cache 改用不含 password hash 的私有 DTO。
- access/refresh JWT 都限制 HS256；Redis session 同时维护可滑动 idle lease 和不可延长 absolute deadline。
- Redis backend unavailable 映射为服务端错误，与 invalid/expired auth 分开。
- Docker YAML secret 默认空值；release 拒绝空、placeholder 或 dummy JWT secret。

## R3 Dependency and CI

- Go toolchain 目标为 1.25.14。
- 最小升级 `golang.org/x/net`、`golang.org/x/text`、`github.com/quic-go/quic-go` 及其必要约束依赖，没有执行全量 `go get -u ./...`。
- Linux GitHub Actions 执行 build、unit、race、vet 和 govulncheck。

## M12 Webook alignment

- 删除未接线的 retry handler、no-op compensate job/compensation experiment 和失效示例文档。
- 保持 `web/handler → service → repository → dao/cache` 以及 `service → outbox → Kafka → consumer → service/repository`。
- 未改变数据库 schema、业务 API 或消息协议。

## Definition of Done

- R1 回归单测与真实 Kafka/Elasticsearch targeted E2E 通过。
- `go build ./...`、`go test ./...`、`go vet ./...`、`govulncheck ./...`、`git diff --check` 通过。
- Windows 不安装额外 C toolchain；race 由 Linux CI 执行。
- 原 `community-v1` 不移动；全部验证后创建 annotated tag `community-v1.1`。
