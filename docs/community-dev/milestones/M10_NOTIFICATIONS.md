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
