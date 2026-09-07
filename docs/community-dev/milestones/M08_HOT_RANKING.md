# M08 Redis 滑动窗口热榜

## 目标

把内容事件聚合到 Redis 分钟桶，并通过物化 snapshot 提供稳定分页的实时热榜。

## 分支

`feat/community-m08-hot-ranking`

## 不做

- 不把权重描述成推荐算法。
- 不新增 MySQL 热榜表或同步写热榜。
- 不接入现有 Retry/DLQ 原型。
- Redis 查询失败时不扫描 MySQL 即时重建。
- 不实现分布式锁；snapshot 使用 Redis Lua 原子创建。

## 输入事件与权重

| Topic | 默认权重 | 来源 |
|---|---:|---|
| `note.published` | 10 | Note + Outbox 同事务 |
| `note.liked` | 3 | 首次 Like + Outbox 同事务 |
| `comment.created` | 5 | Comment + Outbox 同事务 |

配置位于 `hot_rank`：窗口 60 分钟、snapshot TTL 10 分钟、event dedup TTL 168 小时。权重均可配置，修改后需重启进程。

## 写路径

```text
Outbox → Relay → Kafka → user-center-worker
  note.published → Feed handler → HotRank handler
  note.liked --------------------→ HotRank handler
  comment.created ---------------→ HotRank handler

HotRank handler
  → event type / topic validation
  → configured weight
  → Redis Lua:
      EXISTS hot:event:done:{event_id}
      ZINCRBY hot:note:{yyyyMMddHHmm}
      PEXPIREAT bucket
      SET dedup marker PX 168h
```

事件去重、加分、bucket 过期和 done marker 在同一个 Lua 中原子完成。重复投递不会重复加分；Redis 返回丢失后重试也由 done marker 短路。

事件自己的 `occurred_at` 决定 bucket，不使用消费时间。bucket 保留到“bucket 结束 + window + snapshot TTL”；过期事件不会复活历史 bucket。

## 查询路径

```text
GET /rank/hot?limit=20&cursor=
  → HotRankHandler
  → HotRankService.List
  → HotRankRepository
  → RedisHotRankCache
      first page: Lua ZUNIONSTORE recent 60 buckets once
      later page: verify ready marker, read same snapshot ZSET
  → ZREVRANGE snapshot offset..offset+limit
```

第一页生成 `snapshot_minute` 和 `snapshot_minute_offset` cursor。snapshot key 在同一分钟只创建一次；后续到达的事件和窗口滚动不会改变该 snapshot。snapshot 过期后旧 cursor 返回 HTTP 400。

## Redis keys

| Key | Type | TTL |
|---|---|---|
| `hot:note:{yyyyMMddHHmm}` | ZSET，member=note_id，score=分钟内权重和 | window + snapshot grace |
| `hot:event:done:{event_id}` | String | 默认 168h |
| `hot:note:snapshot:{unix_minute}` | ZSET | 默认 10m |
| `hot:note:snapshot:ready:{unix_minute}` | String | 默认 10m |

测试与 benchmark 使用 UUID 隔离 prefix，不读写或删除生产样式 key。

## 失败语义

- Kafka down：API 写入仍按原有事务写 Outbox；热榜保持旧值。Kafka 恢复后 Relay/worker 继续消费，仍在保留期的事件按 event time 回填。
- Hot handler 返回错误：当前分区不提交后续 offset，等待 consumer loop 重建后重试。
- `note.published` Feed 成功而 Hot 失败：重试时 Feed dedupe 跳过，Hot 继续处理。
- Redis down：worker 返回错误且不提交消息；热榜查询返回 500，不访问 MySQL 重建。
- snapshot 过期：旧 cursor 返回明确 400，不静默切换到新窗口。

## 测试与证据

- bucket minute boundary、event weight、invalid event。
- 三 topic handler、topic/type mismatch、handler chain failure。
- duplicate event 原子去重。
- snapshot pagination 在时间推进后保持同一 snapshot。
- empty/expired snapshot、expired bucket。
- 真实 Redis 聚合、late event 不改变 snapshot、bounded polling 到期。
- 60 buckets × 100 notes 的真实 Redis cold/materialized benchmark。

原始结果见 `docs/community-dev/benchmarks/2026-09-08-m08-hot-ranking.md`。

## DoD 结果

- `go build ./...`：PASS。
- `go test ./...`：PASS。
- `go vet ./...`：PASS。
- `git diff --check`：PASS。
- 真实 Redis E2E：PASS。
- `go test -race ./...`：`SKIPPED - local Windows environment has no supported C compiler.`

Race skip 沿用已确认的本地环境限制，不是代码失败；未继续安装系统工具链。
