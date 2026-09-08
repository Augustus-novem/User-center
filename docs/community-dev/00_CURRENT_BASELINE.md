# 00 当前系统基线

`community-base` 是 community 演进前的 backend-only 基线。

## 当前架构

```text
Client
  |
  v
user-center (Gin)
  |---- MySQL
  |---- Redis
  |---- event_outbox
              |
              v
            Relay
              |
              v
            Kafka
           /     \
          v       v
      worker   notification-service
```

## 已验证能力

- 邮箱 Signup 与 Login、JWT access/refresh session。
- Checkin、签到积分、签到 Bitmap、日榜和月榜。
- MySQL 事务内写 Outbox，Relay 投递 Kafka。
- `user.registered` 由 worker 和 notification-service 两个 group 消费。
- `user.activity` 由 worker 消费并写 Redis 行为日志与排行榜。
- Redis Lua 去重、数据库唯一约束、欢迎消息 `SETNX`。
- Docker Compose 同时运行 MySQL、Redis、Kafka、user-center、worker、notification-service。
- 全仓 build/test/vet 与 race tests。

## 当前边界与 Technical Debt

- Outbox Relay 没有多实例抢占保护，也没有专门 retry/backoff。
- 永久非法 Kafka 消息会在 DLQ publish 成功后提交；transient 错误保留原 offset 等待重投。
- notification-service 当前只在 Redis 保存欢迎消息。
- `RankConsistencyCache` 未接入运行时依赖图。
- 当前没有账号级登录限流。
- 没有可引用的 QPS、P95/P99 或缓存命中率基准数据。

以上项目不是 M00 阻塞项；后续只能在对应 milestone 或独立 Technical Debt 任务中处理。
