# M03 Learning Handoff

## 1. 解决的问题

笔记需要可正确、可幂等的点赞和评论。先保证 MySQL 正确性，再用现有 Outbox 把 `note.liked` / `comment.created` 发出去给后续热榜和通知。

## 2. 完整调用链

```text
POST /notes/:id/like
  → EngagementHandler.Like
  → EngagementServiceImpl.Like
       InTx:
         NoteRepository.FindByID
         LikeRepository.Create   UNIQUE(user_id, note_id)
         Outbox note.liked       仅首次插入成功时
  唯一索引冲突 → 成功，不再写 Outbox
```

```text
POST /notes/:id/comments
  → 校验 content
  → InTx: 确认笔记存在 → Insert comments → Outbox comment.created
GET  /notes/:id/comments
  → created_at ASC, id ASC keyset cursor
```

## 3. 核心代码

| Symbol | File | Responsibility |
|---|---|---|
| `EngagementHandler` | `internal/web/engagement.go` | HTTP |
| `EngagementServiceImpl` | `internal/service/engagement.go` | 笔记存在性、幂等 Like、评论校验、Outbox |
| `LikeRepositoryImpl` | `internal/repository/engagement.go` | like 写入/删除 |
| `GORMLikeDAO` | `internal/repository/dao/engagement.go` | 1062 → ErrLikeDuplicate |

## 4. Database

```text
note_likes UNIQUE(user_id, note_id)
comments INDEX(note_id, status, created_at, id)
```

Transactions: 首次 Like 和创建评论与 Outbox 同一事务。重复 Like 的 INSERT 失败不会留下半成品，也不会发重复事件。

## 5. Redis
无

## 6. Kafka

Events/Topics: `note.liked`, `comment.created`
Groups: 无新消费者
Idempotency: Like 靠唯一索引；事件只在首次插入成功时产生。

## 7. Failure scenarios

1. 并发点赞同一笔记：一个 INSERT 成功并发事件，其余 1062 变成功且不发事件。
2. 给已删除笔记点赞：FindByID 映射为笔记不存在。
3. 空评论 / 超长评论：不写库。
4. Outbox 失败：Like/Comment 事务回滚。
5. 重复 Unlike：DELETE 0 行仍成功。

## 8. Trade-offs

- 不用 Redis 计数：M03 先正确后加速；热榜在 M08 再消费事件。
- 重复 Like 不发事件：避免热榜/通知被刷。
- 评论按时间正序：对话阅读；关注列表是倒序。

## 9. Interview questions

1. 为什么重复点赞不能再写 `note.liked`？
2. 唯一索引冲突为什么放在事务外映射成成功？
3. 评论为什么用 ASC cursor，关注为什么用 DESC？
4. 为什么先查笔记再插 like，而不是只靠外键？
5. Unlike 为什么不发事件？
6. 并发 Like 的 check-then-insert 会出什么问题？
7. 评论长度为什么用 rune 而不是 `len(string)`？
8. Kafka 关闭时 Like 还保证幂等吗？
9. 如果 FindByID 在事务外，删除笔记与点赞的窗口是什么？
10. M08 热榜如何从 `note.liked` 重建，而不是读 Redis 当真相？

## 10. 我自己编码的三个任务

1. 给笔记详情加上 `liked_by_me`（当前用户是否点赞过），不要引入 Redis。
2. 写测试：CreateComment 成功后 Publish 失败，评论行必须回滚。
3. 设计 `note.unliked` 要不要加。如果加，热榜扣分如何处理重复消费。

> Agent 不提供本节答案，除非我完成尝试后请求 hint。

## Architecture Review

- [x] web → service → repository → dao
- [x] 复用 NoteRepository 与现有 Transaction/Publisher
- [x] 无新万能包；interface 用于测试与 Wire
- [x] HTTP DTO 与 GORM model 分离
- [x] Like 幂等由唯一索引保证
