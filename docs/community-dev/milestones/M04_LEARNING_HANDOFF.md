# M04 Learning Handoff

## 1. 解决的问题

先做一个正确的关注 Feed baseline：读的时候按关注关系拉笔记。它慢，但语义简单，后面 Push/Hybrid 必须拿它对照。

## 2. 完整调用链

```text
GET /feed/following?cursor=&limit=
  → JWT
  → FeedHandler.Following
  → FeedServiceImpl.ListFollowing
  → FeedRepository.ListFollowing
  → GORMFeedDAO.ListFollowingNotes
       notes n JOIN user_relations r
         ON r.followee_id = n.author_id
        AND r.follower_id = current_user
       WHERE n.status = published
         AND (n.created_at, n.id) < cursor
       ORDER BY n.created_at DESC, n.id DESC
```

没有 Kafka，没有 Redis Inbox。

## 3. 核心代码

| Symbol | File | Responsibility |
|---|---|---|
| `FeedHandler` | `internal/web/feed.go` | HTTP |
| `FeedServiceImpl` | `internal/service/feed.go` | cursor 编解码、limit+1 |
| `GORMFeedDAO` | `internal/repository/dao/feed.go` | JOIN + keyset |

## 4. Database

Tables: 复用 `notes`、`user_relations`。无新表。

Indexes used:

```text
user_relations UNIQUE(follower_id, followee_id)
notes (author_id, status, created_at, id)
```

## 5. Redis
无

## 6. Kafka
无新事件。

## 7. Failure scenarios

1. 无关注：JOIN 为空列表。
2. 同毫秒多篇笔记：用 id 打破并列。
3. 翻页后插入更新笔记：新笔记 created_at 更大，不会进入旧 cursor 页，因此不重复。
4. 软删笔记：status 过滤，不出现。
5. 非法 cursor：业务错误，不拼进 SQL。

## 8. Trade-offs

- JOIN Pull 发布成本低、读成本随关注人数上升。这就是 baseline。
- 不预先 fanout：没有 M05 的写放大，也没有 Inbox 一致性问题。
- 本阶段不记录假 P95。没有压测环境输出就不能写数字。

## 9. Interview questions

1. 为什么 Feed 默认 cursor 而不是 OFFSET？
2. 两页之间插入新笔记，keyset 为什么不重复？
3. OFFSET 在同一场景为什么会重复或跳过？
4. 为什么用 JOIN 而不是先查出 following ids 再 `IN (...)`？
5. 自己的笔记为什么不在 Following Feed 里？
6. Pull Feed 的读放大发生在哪？
7. 为什么 deleted 必须在 SQL 过滤而不是应用层丢掉？
8. 相同 created_at 只用时间做 cursor 会怎样？
9. M05 Push 相对这个 baseline 改善的是读还是写？
10. 没有 P95 数据时，能不能说 Pull 一定比 Push 慢？

## 10. 我自己编码的三个任务

1. 给 Feed item 补上作者是否被当前用户关注的冗余字段，并说明它为什么不必再查一次关系表。
2. 写一个测试：page1 之后删除一篇旧笔记，page2 既不重复也不把已读的更新笔记塞回来。
3. 用 `EXPLAIN` 看这条 JOIN（本地 MySQL）。把结果贴进 `templates/BENCHMARK_RECORD.md`，不要编数字。

> Agent 不提供本节答案，除非我完成尝试后请求 hint。

## Architecture Review

- [x] web → service → repository → dao
- [x] 无 Kafka fanout / Redis Inbox
- [x] HTTP DTO 复用 note VO，不直接暴露 GORM model
- [x] 无新万能包
- [x] 未编造性能数字
