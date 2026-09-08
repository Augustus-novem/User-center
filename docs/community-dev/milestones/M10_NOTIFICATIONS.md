# M10 Notification 业务化

## 目标
把现有 welcome notification-service 演进为真实社区通知模块。

## 分支
`feat/community-m10-notification`

## Events
`user.followed`、`note.liked`、`comment.created`。

## 数据
建议真正落 MySQL：

```text
notifications
- id
- receiver_id
- actor_id
- type
- biz_id
- is_read
- created_at
```

Redis 只做可选缓存，不作唯一存储。

## API
`GET /notifications`、`POST /notifications/:id/read`。

## 幂等
事件 event_id 去重 + 必要 DB 唯一约束。

## 必测
duplicate event、actor=receiver 规则、read idempotency、cursor pagination。

## 实现结果

### 调用链

```text
POST follow / like / comment
  → service MySQL transaction
  → business row + event_outbox
  → Relay → Kafka
  → notification-service
  → CommunityHandler → NotificationService
  → notifications (MySQL)

GET /notifications
  → NotificationHandler → NotificationService
  → NotificationRepository → MySQL stable cursor query

POST /notifications/{id}/read
  → current user scoped update
  → repeated request remains successful
```

### Database

- `notifications`：`id`、`event_id`、`receiver_id`、`actor_id`、`type`、`biz_id`、`is_read`、`created_at`。
- 唯一索引 `uk_notification_event(event_id)` 是 Kafka 重复投递的最终幂等保障。
- 列表索引 `idx_notification_receiver_created(receiver_id, created_at, id)` 支持 `(created_at,id)` 降序稳定游标。
- `actor_id == receiver_id` 的关注/点赞/评论事件被视为成功但不写通知。
- 笔记通知接收者从 MySQL note author 读取；即使笔记稍后软删除，历史事件仍可解析接收者。

### Kafka

- 新增 `user.followed`：关注关系与事件 Outbox 在同一事务；重复关注不生成新事件。
- notification-service 保留 `user.registered` welcome handler，并新增消费 `user.followed`、`note.liked`、`comment.created`。
- 三种社区事件与 welcome 共用 `user-center-notification-service` consumer group，但使用独立 handler。

### API

- `GET /notifications?cursor={created_at}_{id}&limit=20`：只返回当前登录用户通知，默认 20、最大 50。
- `POST /notifications/:id/read`：只允许 receiver 更新；他人通知统一返回不存在；重复标记成功。
- API 不暴露内部 `event_id`。

### 当前边界

- MySQL 是社区通知唯一真相源；本阶段不增加通知 Redis cache。
- 没有通知聚合、未读计数、删除、邮件/短信/移动推送。
- 沿用现有 consumer 的失败处理和 offset 语义；不在 M10 提前实现 Retry/DLQ。

## 验证结果（2026-09-08）

- `go test -tags=e2e -run '^TestNotification_CommunityEventsThroughKafkaE2E$' -count=1 -v .`：PASS（13.87s）。
- E2E 覆盖 Follow/Like/Comment → Outbox → Relay → Kafka → MySQL、self-action 忽略、event 重放去重、游标翻页和重复已读。
- 其余 M10 DoD 结果记录在 `M10_LEARNING_HANDOFF.md`。
