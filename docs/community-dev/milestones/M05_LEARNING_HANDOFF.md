# M05 Learning Handoff

## 1. 解决的问题

把 Following Feed 从 M04 的请求时 JOIN，改成发布时扇出到 Redis Inbox。读路径变成 Inbox candidate + MySQL 过滤，写路径承担粉丝分页 fan-out。

## 2. 完整调用链

写：

```text
POST /notes
  → NoteService.Publish（同一事务写 notes + event_outbox note.published）
  → Outbox Relay
  → Kafka topic note.published
  → worker group user-center-worker
  → NotePublishedHandler
       TryBegin(event_id)  namespace=worker:note_published
       FeedService.FanoutPublished
         loop ListFollowers(cursor, fanout_batch_size)
         pipeline Lua ZADD+trim → feed:inbox:{follower_id}
       全部 batch 成功后 MarkDone；失败 ClearInFlight，不提交 Kafka offset
```

读：

```text
GET /feed/following?cursor=&limit=
  → JWT
  → FeedHandler.Following
  → FeedService.ListFollowing
  → Redis ZREVRANGEBYSCORE feed:inbox:{user_id}  (score=note_id DESC)
  → NoteRepository.FindByIDs
  → 丢掉 missing / deleted / unpublished
  → FollowRepository.ListFolloweeIDs 丢掉已取关的 stale candidate
  → 不足一页继续扫 Inbox，不因脏 candidate 报错
cursor = 本页最后一条有效笔记的 note_id
```

## 3. 核心代码

| Symbol | File | Responsibility |
|---|---|---|
| `RedisFeedInbox` | `internal/repository/cache/feed_inbox.go` | pipeline Lua 写入与 Inbox 分页 |
| `feed_inbox_add.lua` | `internal/repository/cache/lua/feed_inbox_add.lua` | 原子 ZADD + 超限裁剪 |
| `FeedServiceImpl.FanoutPublished` | `internal/service/feed.go` | 粉丝 cursor 分批扇出 |
| `FeedServiceImpl.ListFollowing` | `internal/service/feed.go` | Inbox hydrate 与脏 candidate 过滤 |
| `NotePublishedHandler` | `internal/worker/note_published_handler.go` | event_id 幂等 + 失败可重试 |
| `InitFeedService` | `ioc/feed.go` | 注入 batch/maxItems，业务层不读 Viper |

## 4. Database

Tables: 无新表。复用 `notes`、`user_relations`、`event_outbox`。

新增查询：

```text
notes WHERE id IN (...)
user_relations WHERE follower_id=? AND followee_id IN (...)
user_relations 粉丝列表继续用 (created_at, id) DESC cursor
```

Pull JOIN DAO（`GORMFeedDAO`）保留给 M06，本 milestone 读路径不再调用它。

## 5. Redis

| Key | 类型 | 用途 |
|---|---|---|
| `feed:inbox:{user_id}` | ZSET | member=note_id, score=note_id |
| `consumer:event:processing:worker:note_published:{event_id}` | String | 处理中 |
| `consumer:event:done:worker:note_published:{event_id}` | String | 完成，TTL 7d |

Inbox 无 TTL。容量由 `feed.inbox_max_items`（默认 500）Lua 裁剪。

## 6. Kafka

| Item | Value |
|---|---|
| Event | `note.published` |
| Topic | `note.published` |
| Group | `user-center-worker` |
| Idempotency | `event_id` + Redis Lua deduper |

## 7. Failure scenarios

1. 作者 0 粉丝：fanout 成功，不写 Redis，仍 MarkDone。
2. 某一 follower batch 写 Redis 失败：ClearInFlight，Kafka 不提交；重试会再扫全部粉丝，ZADD 同 note_id 不重复。
3. 同一 event_id 重复投递：TryBegin 失败，跳过扇出。
4. 笔记已删或 Inbox 里还有已取关作者：读路径静默跳过，不返回错误。
5. Inbox 超过上限：Lua 删掉最低分（更旧 note_id），与写入同一脚本。

## 8. Trade-offs

- score=note_id 而不是 timestamp：当前发布即 INSERT，自增 ID 是严格稳定顺序；避免同毫秒并列和 float64 复合 score。草稿改 ID 后再发布时这个假设失效，见 ADR-008。
- 重试全量扫粉丝而不是记录 fanout cursor：实现简单，依赖 ZADD 幂等。大粉丝数会重复写已成功的 batch。
- 读路径过滤 unfollow，而不是写路径立刻删 Inbox：取消关注保持同步、便宜；Inbox 允许短暂脏数据。
- 不在 M05 做 celebrity threshold：避免把 Hybrid 复杂度提前塞进第一版 Push。

## 9. Interview questions

1. 为什么 Inbox score 用 note_id 而不是发布时间？
2. Redis ZSET score 是 float64，为什么不能把 timestamp 和 id 编码进同一个 score？
3. 部分 batch 成功后失败，为什么还可以安全重试？
4. 为什么必须全部 batch 成功才能 MarkDone / 提交 Kafka offset？
5. event_id 幂等和 ZADD member 幂等各挡住什么重复？
6. 取消关注后 Inbox 里的旧 candidate 为什么还能被读路径正确丢掉？
7. 为什么粉丝必须 cursor 分页，而不是一次查出全部 follower_id？
8. Inbox trim 为什么放进 Lua，而不是 ZADD 后再单独 ZREMRANGEBYRANK？
9. Push Feed 改善的是读还是写？对大 V 会引入什么问题？
10. 没有压测数字时，能不能说 Push 一定比 Pull 快？

## 10. 我自己编码的三个任务

1. 给 fanout 增加“已成功写到哪个 follower cursor”的进度，使重试不必从第一页粉丝重扫，并证明失败窗口仍 at-least-once。
2. 读路径过脏 candidate 时写一条带 `note_id`/`user_id` 的 debug 日志，确认不会把主库打满。
3. 把 Inbox cursor 从 note_id 换成你自己设计的 tie-break，并说明它为什么比当前 ADR-008 更复杂或更差。

> Agent 不提供本节答案，除非我完成尝试后请求 hint。

## Architecture Review

- [x] web 只依赖 FeedService。
- [x] service 不依赖 Gin/GORM/Viper/Wire；batch/max 由 ioc 注入。
- [x] repository 不依赖 web/service。
- [x] Redis Lua 留在 cache adapter；Kafka handler 在 worker。
- [x] `FeedInbox` / `PublishedNoteFanout` 是真实模块边界，不是机械 IXXX。
- [x] IoC 在 `ioc/feed.go` + Wire + `cmd/worker` 手工装配。
- [x] 幂等复用现有 Deduplicator，没有复制一套 Lua。
- [x] HTTP DTO 仍是 noteVO；GORM model 未泄漏到 handler。
- [x] 无新万能 package，无 celebrity/ES/热榜。
