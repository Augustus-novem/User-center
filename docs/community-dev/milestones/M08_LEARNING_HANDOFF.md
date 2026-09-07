# M08 Learning Handoff

## 1. 解决的问题

把 publish/like/comment 的异步事件转换为可重建的内容热度派生数据，并用稳定 snapshot 避免滑动窗口分页重复或遗漏。

## 2. 完整调用链

```text
MySQL business transaction + Outbox
  → Relay → Kafka → worker HotRankHandler
  → HotRankService.Record → HotRankRepository
  → Redis Lua minute bucket + event dedup

GET /rank/hot
  → HotRankHandler → HotRankService.List
  → materialize/read Redis snapshot
  → stable offset cursor page
```

## 3. 核心代码

| Symbol | File | Responsibility |
|---|---|---|
| `HotRankConfig` | `internal/config/type.go` | window、TTL、三类权重 |
| `RedisHotRankCache.Record` | `internal/repository/cache/hot_rank.go` | event-time bucket 与原子写入 |
| `create_hot_rank_snapshot.lua` | `internal/repository/cache/lua/` | 一次性聚合窗口与 ready marker |
| `HotRankServiceImpl` | `internal/service/hot_rank.go` | 权重映射、cursor、page rank |
| `HotRankHandler` | `internal/worker/hot_rank_handler.go` | Kafka envelope 校验与消费 |
| `ChainHandlers` | `internal/worker/hot_rank_handler.go` | note.published Feed/Hot 顺序组合 |
| `web.HotRankHandler` | `internal/web/hot_rank.go` | `GET /rank/hot` |

## 4. Database

无新表、索引或查询。Note/Like/Comment 与 Outbox 的原有事务边界不变。

## 5. Redis

- 分钟 bucket：`hot:note:{yyyyMMddHHmm}` ZSET。
- 去重：`hot:event:done:{event_id}` String。
- snapshot：`hot:note:snapshot:{unix_minute}` ZSET。
- ready marker：`hot:note:snapshot:ready:{unix_minute}` String。

Redis 是可重建派生数据，不是内容或互动真相源。

## 6. Kafka

Topics：`note.published`、`note.liked`、`comment.created`。

Group：`user-center-worker`。同一事件可能重复投递；Hot Lua 使用 `event_id` 原子去重。没有接入 retry topic 或 DLQ handler。

## 7. Failure scenarios

1. Kafka down：Outbox 保留，榜单暂时陈旧。
2. Redis write down：handler 返回错误，offset 不提交。
3. duplicate event：Lua done marker 阻止二次 ZINCRBY。
4. event 很晚才到：按 occurred_at 写 bucket；已过保留期则立即到期。
5. snapshot 创建并发：Lua 只允许同 minute 首次物化。
6. snapshot 空榜：ready marker 仍存在，不反复重建。
7. snapshot 过期：旧 cursor 返回 invalid/expired。
8. Feed 成功而 Hot 失败：独立幂等让重试只补 Hot。

## 8. Trade-offs

- 分钟桶让窗口淘汰简单，但窗口精度只有一分钟。
- snapshot 提供稳定分页，代价是短期额外 ZSET 内存和最多 10 分钟的新鲜度隔离。
- 权重明确可解释但很粗糙；不是个性化排序。
- event dedup TTL 有界；超出 TTL 的极晚重复事件通常对应已过期 bucket。
- Redis down 时不做 MySQL fallback，避免榜单查询把数据库拖垮。

## 9. Interview questions

1. 为什么用 occurred_at 而不是消费时间选择 bucket？
2. 为什么去重与 ZINCRBY 必须在同一 Lua？
3. 两阶段 TryBegin/MarkDone 为什么可能重复加分？
4. 分钟桶相对单一大 ZSET 有什么淘汰优势？
5. 只固定 snapshot_minute 而不物化结果，为什么仍可能分页重复？
6. ready marker 为什么必须与 snapshot ZSET 分开？
7. Kafka down 时为什么业务写仍能成功？
8. Redis down 为什么不实时扫 MySQL 重建榜单？
9. 同分 note 的顺序由什么保证稳定？
10. 本地 Redis benchmark 能证明什么、不能证明什么？

## 10. 我自己编码的三个任务

1. 写一个相同 score 的分页测试，验证 Redis member tie-break 在 snapshot 内稳定。
2. 给 HotRankHandler 增加 malformed JSON 表驱动测试，并区分解析失败和业务字段失败。
3. 计算 window_minutes 从 60 调成 120 后 bucket retention 和 snapshot key 数量如何变化。

> Agent 不提供本节答案，除非我完成尝试后请求 hint。

## Architecture Review

- [x] web → service → repository → cache。
- [x] service 不依赖 Gin、Redis、Viper 或 config package。
- [x] 权重由 IOC/worker composition root 显式映射。
- [x] Kafka handler 依赖最小 `HotRankRecorder`。
- [x] 原子性集中在 Redis Lua，没有复制到业务函数。
- [x] 无新 DB schema、全局 mutable dependency、Retry/DLQ 或推荐系统。
