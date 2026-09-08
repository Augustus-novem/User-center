# M10 Learning Handoff

## 1. 解决的问题

把只写 Redis welcome message 的 notification-service 扩展成社区站内通知服务，同时保留现有欢迎链路。关注、点赞和评论事件异步写入 MySQL，HTTP API 只访问当前用户数据。

## 2. 完整调用链

```text
Follow/Like/Comment Service
  → business transaction + Outbox
  → Relay → Kafka
  → notification-service CommunityHandler
  → NotificationService
  → NotificationRepository → MySQL notifications

authenticated HTTP
  → NotificationHandler
  → NotificationService
  → NotificationRepository → MySQL
```

## 3. 核心代码

| Symbol | File | Responsibility |
|---|---|---|
| `UserFollowedEvent` | `internal/events/user_followed.go` | 关注事件 envelope 与 event ID |
| `FollowServiceImpl.Follow` | `internal/service/follow.go` | 关注关系与 Outbox 原子写入 |
| `NotificationOfDB` | `internal/repository/dao/notification.go` | 通知表、唯一键、列表和已读 SQL |
| `NotificationRepositoryImpl` | `internal/repository/notification.go` | persistence/domain 映射与 note author 解析 |
| `NotificationServiceImpl` | `internal/service/notification.go` | self-action、通知映射、游标和已读规则 |
| `CommunityHandler` | `internal/notification/community_handler.go` | Kafka 解码、类型校验与 Service 调用 |
| `NotificationHandler` | `internal/web/notification.go` | 当前用户列表和已读 API |

## 4. Database

- 表：`notifications`。
- 唯一索引：`uk_notification_event(event_id)`。
- 列表索引：`idx_notification_receiver_created(receiver_id, created_at, id)`。
- `event_id` 解决相同 Kafka event 的重复投递；它不会去重合法的 unfollow/refollow 或 unlike/relike，因为新业务动作具有新 event ID。
- `created_at` 使用事件发生时间，`id` 解决同毫秒通知的确定性排序。

## 5. Kafka

- Topics：`user.followed`、`note.liked`、`comment.created`。
- Group：`user-center-notification-service`。
- Producer：业务事务内写 Outbox，Relay 以 at-least-once 发送。
- Consumer：成功落库或命中 DB 唯一键后才返回成功；错误不提交当前 offset。

## 6. Redis

- 社区通知没有新增 Redis key，MySQL 是真相源。
- 原 `welcome:message:user:{user_id}` 与 welcome consumer idempotency keys 保留，不迁移、不混用。

## 7. Failure scenarios

1. Outbox 写入失败：关注/点赞/评论业务事务整体回滚。
2. Kafka 重复投递：`event_id` 唯一键使第二次 insert 变成 no-op。
3. 自己操作自己的内容：consumer 成功处理，但不生成通知。
4. 笔记事件到达前笔记已软删除：直接读取 DB note author，仍能写入历史通知。
5. 不存在或属于他人的通知标记已读：按 receiver 条件查找，统一返回通知不存在。
6. notification-service 暂停：不影响 HTTP 业务事务；恢复后从 consumer group offset 继续消费。

## 8. Trade-offs

- 每个事件生成一条通知，模型简单可审计，但尚未做同类通知聚合。
- 列表直接读 MySQL，避免把 Redis 变成通知真相源；大规模未读计数与缓存留到有证据时再设计。
- comment 的 `biz_id` 指向 comment，like 的 `biz_id` 指向 note，follow 的 `biz_id` 指向 actor；单字段便于通用列表，但客户端需按 type 解释。
- consumer 在消费时读取 note author，多一次 DB 查询，换取事件 schema 简洁和服务端对接收者规则的集中控制。

## 9. Architecture Review

- [x] HTTP 保持 web → service → repository → dao。
- [x] 异步保持 service → Outbox → Kafka → consumer handler → service → repository。
- [x] Gin、GORM、Sarama 分别停留在 web、dao、consumer 层。
- [x] IoC 集中在 Wire/ioc 与 notification command composition root。
- [x] 无 service locator、全局 mutable dependency、万能 package 或新基础设施。
- [x] transaction boundary 在 Follow/Engagement service 可见。
- [x] DB 唯一约束而非进程内判断承担最终幂等。

## 10. Verification

- build / unit test / vet / diff check：PASS。
- 真实 MySQL + Kafka E2E：PASS（13.87s）。
- E2E 验证三类事件、self-action、重复投递、稳定 cursor 与 read idempotency。
- race：SKIPPED（本地 Windows 无受支持 C 编译器；按用户约束不修改系统环境）。

## 11. 我自己编码的三个任务

1. 为 `NotificationDAO.MarkRead` 写 sqlmock 用例，分别覆盖首次更新、已读 no-op 和跨用户不存在。
2. 为通知列表增加一个同毫秒 5 条数据、连续翻 3 页的 repository 集成测试。
3. 设计“未读数”查询的最小 SQL 与索引评估，只写说明，不引入 Redis cache。

> Agent 不提供本节答案，除非我完成尝试后请求 hint。
