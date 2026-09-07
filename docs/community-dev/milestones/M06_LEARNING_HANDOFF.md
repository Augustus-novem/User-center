# M06 Learning Handoff

## 1. 解决的问题

Push Feed 对大 V 会把一次发布放大成 N 次 Inbox 写。Hybrid 用 follower 数阈值把大 V 留在 Pull，把普通作者留在 Push。

## 2. 完整调用链

写：

```text
note.published
  → CountFollowers(author)
  → count >= feed.fanout_threshold（且 threshold>0）：跳过扇出，MarkDone
  → otherwise：M05 粉丝 cursor 分批 ZADD Inbox
```

读：

```text
GET /feed/following
  → Inbox hydrate（删帖/未发布/取关过滤）
  → page following，FilterIDsByMinFollowers
  → 每个 celebrity ListPublishedBefore(note_id)
  → 按 note_id DESC merge 去重
cursor = 本页最后一条有效笔记 note_id
```

## 3. 核心代码

| Symbol | File | Responsibility |
|---|---|---|
| `FeedConfig.CelebrityThreshold` | `internal/config/type.go` | 阈值配置，<=0 关闭 Hybrid |
| `CountFollowers` | `internal/repository/dao/follow.go` | 发布时判定是否大 V |
| `FilterIDsByMinFollowers` | `internal/repository/dao/follow.go` | 对当前 following 页做 COUNT 过滤 |
| `ListPublishedBefore` | `internal/repository/dao/note.go` | 大 V 近况按 note_id Pull |
| `FeedServiceImpl.FanoutPublished` | `internal/service/feed.go` | 阈值短路或扇出 |
| `FeedServiceImpl.ListFollowing` | `internal/service/feed.go` | Inbox + celebrity merge |

## 4. Database

无新表。新增查询走现有 `user_relations(followee_id, ...)` 与 `notes(author_id, status, ...)`。

celebrity 判定每次按 MySQL COUNT，不引入 Redis 集合作为真相。

## 5. Redis

无新 key。Inbox 仍是 `feed:inbox:{user_id}`。大 V 新笔记不写入粉丝 Inbox。

## 6. Kafka

无新 topic。`note.published` 仍消费；大 V 事件 MarkDone 但不扇出。

## 7. Failure scenarios

1. 普通作者：count < threshold，行为等于 M05。
2. 大 V：不写 Inbox，读路径靠 Pull。
3. 同一笔记同时在 Inbox 与 Pull：merge 去重。
4. 新关注大 V：Pull 能读到其近期笔记，不依赖历史 Inbox。
5. 取消关注大 V：following 页不再包含该作者，Pull 与 Inbox 脏数据都不会出现。
6. 分页后插入更高 note_id：不会进入旧 cursor 页。
7. `fanout_threshold<=0`：关闭 Hybrid，全量扇出，读路径不 Pull。

## 8. Trade-offs

- 阈值默认 1000 只是本地可调起点，不是压测结论。M11 之前不能把 1000 写成最优值。
- 读路径按批扫描 following 并 COUNT，关注很多大 V 的用户读成本上升。换 Redis celebrity set 会让 COUNT 与缓存双真相。
- 作者从普通变成大 V 后，旧 Inbox 仍在；新笔记不再扇出。merge 去重处理重叠。
- 作者从大 V 降回普通后，历史缺口只靠之后的 Inbox 填，本版不回填。

## 9. Interview questions

1. 为什么用 follower COUNT 而不是写死 1 万粉？
2. threshold 从 1000 改成 5000，已经在 Inbox 里的笔记怎么办？
3. 为什么大 V 跳过扇出后仍要 MarkDone？
4. 新关注大 V 为什么立刻能看到近况？
5. merge 为什么继续用 note_id 而不是 created_at？
6. 同一 note 既在 Inbox 又被 Pull 到，如何证明不重复？
7. following 很多时，为什么还要分页而不是一次 JOIN 出所有 celebrity？
8. 把 celebrity 放进 Redis SET 的主要风险是什么？
9. Hybrid 改善的是谁的写放大、牺牲的是谁的读？
10. 没有 benchmark 时能不能说 1000 比 10000 更好？

## 10. 我自己编码的三个任务

1. 给 `FilterIDsByMinFollowers` 写 sqlmock，断言 `GROUP BY followee_id HAVING COUNT(*) >= ?`。
2. 模拟作者粉数跨过 threshold 前后各发一篇，画出 Inbox 与 Pull 各自有哪些 note_id。
3. 把 celebrity follow 扫描页大小改成可配置，并说明为什么不要在 service 里写死 50。

> Agent 不提供本节答案，除非我完成尝试后请求 hint。

## Architecture Review

- [x] web 未改，仍只调 FeedService。
- [x] 阈值由 ioc 从 FeedConfig 注入，service 不读 Viper。
- [x] COUNT / Pull 查询留在 dao，repository 只做映射。
- [x] 无新万能 package，无 ES/热榜。
- [x] 没有编造性能数字。
