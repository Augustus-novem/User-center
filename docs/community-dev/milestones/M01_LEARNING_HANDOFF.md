# M01 Learning Handoff

## 1. 解决的问题

社区 Feed 需要真实的关注关系。M01 只把“谁关注了谁”做成可查询、可幂等写入的 MySQL 数据，不引入缓存或事件。

## 2. 完整调用链

```text
POST /users/:id/follow
  → JWT middleware（非公开路径，必须登录）
  → web.FollowHandler.Follow
  → service.FollowServiceImpl.Follow
       拒绝 follower_id == followee_id
       UserRepository.FindByID 确认 followee 存在
       FollowRepository.Create
  → dao.GORMFollowDAO.Insert
  → MySQL user_relations
       UNIQUE(follower_id, followee_id) 冲突 → ErrFollowDuplicate → HTTP 仍返回成功
```

```text
GET /users/:id/following?cursor=&limit=
  → FollowHandler.Following
  → FollowServiceImpl.ListFollowing
       解析 cursor = created_at + "_" + id
       向 repository 多取 1 条判断 has_more
  → GORMFollowDAO.listBy(follower_id)
       WHERE follower_id = ?
         AND (created_at < cursor.ts OR (created_at = cursor.ts AND id < cursor.id))
       ORDER BY created_at DESC, id DESC
```

取消关注是单行 DELETE；0 行也视为成功。

## 3. 核心代码

| Symbol | File | Input | Output | Responsibility |
|---|---|---|---|---|
| `FollowHandler` | `internal/web/follow.go` | HTTP + JWT user | JSON DTO | 参数校验、错误映射 |
| `FollowServiceImpl` | `internal/service/follow.go` | follower/followee/cursor | domain 错误或 `FollowPage` | 自关注、用户存在性、幂等、游标编解码 |
| `FollowRepositoryImpl` | `internal/repository/follow.go` | IDs / cursor | domain.UserRelation | DAO ↔ domain 转换 |
| `GORMFollowDAO` | `internal/repository/dao/follow.go` | persistence model | MySQL rows | 唯一索引冲突映射、keyset 查询 |
| `UserRelationOfDB` | `internal/repository/dao/follow.go` | — | `user_relations` | 表/索引定义 |

## 4. Database

Tables:

```text
user_relations(id, follower_id, followee_id, created_at)
```

Indexes:

```text
PRIMARY KEY(id)
UNIQUE uk_user_relation_follower_followee(follower_id, followee_id)
INDEX idx_follower_created(follower_id, created_at, id)
INDEX idx_followee_created(followee_id, created_at, id)
```

Transactions: 单行 INSERT/DELETE，不需要跨表事务。用户存在性检查与插入不是同一事务；followee 在检查后被删属于可接受的竞态，唯一索引仍保证不会出现重复关系。

## 5. Redis

Keys: 无
Structures: 无
TTL: 无

M01 明确不加 Redis。关注数、关系缓存留给后续有 baseline 后再做。

## 6. Kafka

Events: 无
Topics: 无
Groups: 无
Idempotency: 由 MySQL 唯一索引承担，不走 Outbox。

关注关系变化事件会在 Push Feed 需要 fan-out 时再加，避免现在发没有消费者的消息。

## 7. Failure scenarios

1. 并发重复关注：两个请求同时 INSERT，一个成功，一个收到 1062，service 把它当成已关注。
2. 关注已删除用户：FindByID 返回 not found，接口返回业务错误，不写关系。
3. 取消从未关注过的用户：DELETE 影响 0 行，接口仍成功。
4. 非法 cursor：service 返回 `ErrInvalidCursor`，不会把字符串拼进 SQL。
5. 列表在分页期间插入更新的关注：keyset 以 `(created_at, id)` 向前推进，更新的行不会让旧页重复；offset 分页做不到这一点。

## 8. Trade-offs

- 用唯一索引而不是先 SELECT 再 INSERT：避免 check-then-insert 竞态，应用层只处理冲突。
- 用 cursor 而不是 `LIMIT/OFFSET`：关注列表会持续插入，offset 会跳过或重复。
- 列表不回表用户资料：M01 只要关系图；资料聚合会放大查询，留给 Note/Feed。
- 不发 Kafka：当前没有消费者，发了只会增加 Outbox 负担。

## 9. Interview questions

1. 为什么关注幂等必须靠唯一索引，而不是 `if exists { return }`？
2. 唯一索引冲突时为什么返回成功而不是 409？
3. 为什么 cursor 要同时带 `created_at` 和 `id`？
4. 同一毫秒关注两个人时，只用时间戳做 cursor 会发生什么？
5. followers 和 following 为什么需要两套复合索引？
6. 关注自己为什么放在 service 而不是数据库 CHECK？
7. FindByID 和 INSERT 不在同一事务，可能出现什么窗口？
8. 为什么 M01 不把关注数放进 Redis？
9. 为什么现在不发 `user.followed` 事件？
10. offset 分页在高并发插入下的具体故障是什么？

## 10. 我自己编码的三个任务

1. 给 `GET /users/:id/following` 增加可选 `order=asc`，要求仍然使用稳定 cursor，不能改成 offset。
2. 写一个测试：同一 `created_at` 下插入 3 条关系，`limit=1` 连续翻 3 页，断言每页 `user_id` 不重复、不遗漏。
3. 如果 followee 在 FindByID 成功后、INSERT 前被删除，当前实现会怎样？先口述，再决定要不要把存在性检查放进同一事务。

> Agent 不提供本节答案，除非我完成尝试后请求 hint。

## Architecture Review

- [x] web 只向 service/必要 contract 依赖。
- [x] service 不依赖 Gin/GORM/Viper/Wire。
- [x] repository 不依赖 web/service。
- [x] domain 不依赖基础设施 SDK。
- [x] 无 import cycle。
- [x] 新文件落在现有 `web/service/repository/dao`，无新 package。
- [x] interface 用于测试替身和 Wire 边界，无 `IFollowXxx`。
- [x] 依赖由 constructor + Wire 注入。
- [x] 登录校验复用现有 JWT middleware。
- [x] HTTP DTO 与 GORM model 分开。
- [x] 单行写操作的事务边界可从 service 看懂。
- [x] 无 utils/common/helpers，无无关重构。
