# user-center 项目面试说明

本文只描述当前运行入口真实接入并经过测试或 E2E 验证的能力。仓库中存在但未接入的实验代码，不作为“已实现功能”介绍。

## 一分钟项目介绍

这是一个 Go 用户中心后端，使用 Gin、GORM、MySQL、Redis 和 Kafka。主服务负责注册、登录、JWT 会话、签到和排行榜 API；业务事务同时写入 MySQL Outbox，由 Relay 投递 Kafka。worker 处理欢迎积分、签到行为日志和排行榜，notification-service 把欢迎消息保存到 Redis。

它采用轻量拆分，不是完整微服务体系。重点是数据库事务、Outbox、at-least-once 消费、幂等、Redis 数据结构和清晰的依赖装配。

## 架构与边界

### 为什么拆 worker 和 notification-service？

- HTTP 主链路只处理必须同步完成的事务。
- 欢迎积分、欢迎消息、行为日志和排行榜适合异步后处理。
- 不同 Consumer Group 可以分别处理同一条注册事件。
- 当前没有服务发现、RPC 治理或分布式事务框架，不把项目包装成大型微服务。

### 三个运行入口分别做什么？

- `user-center`：HTTP API、MySQL/Redis 访问、Outbox 写入与 Relay。
- `worker`：消费 `user.registered` 和 `user.activity`。
- `notification-service`：消费 `user.registered`，写 Redis 欢迎消息。

旧的 no-op `compensate-job` 实验已在 community-v1.1 清理；当前不宣称存在通用补偿任务。

## Signup 链路

### 注册成功时发生了什么？

1. Handler 校验邮箱、密码格式及两次密码一致性。
2. Service 对密码做 bcrypt hash。
3. MySQL 事务写用户和 `user.registered` Outbox 记录。
4. Relay 把 Outbox 事件投递到 Kafka，并把记录标记为 `published`。
5. worker 通过数据库唯一约束幂等写入 20 分欢迎积分。
6. notification-service 通过 Redis `SETNX` 保存欢迎消息。

### 为什么使用 Outbox？

直接“先写数据库、再发 Kafka”会在两步之间留下不一致窗口。Outbox 让业务数据和待发布事件进入同一个 MySQL 事务，再由 Relay 异步发送。

Outbox 提供的是 at-least-once 基础，不是 exactly-once。发送成功但状态回写失败时可能重复发送，因此消费者仍需幂等。

## Login 与 JWT

### 登录流程是什么？

- 按邮箱查询用户并校验 bcrypt 密码。
- 登录成功后生成 access token 和 refresh token。
- refresh session/JTI 保存到 Redis。
- access token 从响应头 `x-jwt-token` 返回；受保护接口使用 `Authorization: Bearer ...`。
- JWT claims 绑定登录时的 User-Agent，并在请求中校验 Redis session。

### 当前有哪些限流？

- 短信验证码链路有 Redis Lua 限流与校验逻辑。
- HTTP 层可以启用配置驱动的全局 Redis 滑动窗口限流。
- 当前没有接入账号级登录限流，不能描述为“双层登录防爆破”。

## Checkin 与排行榜

### 签到事务包含什么？

- 写签到记录和连续签到统计。
- 写 5 分签到积分流水。
- Kafka 开启时，在同一事务中写 `user.activity` Outbox 记录。
- 同步刷新签到 Bitmap 缓存；失败只记录日志，MySQL 仍是真相来源。

### worker 如何处理签到事件？

一个 Redis Lua 脚本原子完成：

- `worker:event:done:{event_id}` 去重。
- 把行为 JSON 写入 `activity:log:user:{user_id}`。
- 更新 `rank:active:daily:{yyyyMMdd}`。
- 累加 `rank:active:monthly:{yyyyMM}`。

日榜 score 同时编码积分和当天剩余时间，用于同分时让更早签到者靠前；月榜累计签到积分。

### Kafka 关闭时会怎样？

主服务使用 Nop publisher，不写事件；签到后置动作回退为同步更新排行榜与行为日志。worker 和 notification-service 在 Kafka 关闭时不会启动。

## Kafka 与消费语义

### 当前 topic 和 group 是什么？

| Topic | Group | 处理 |
|---|---|---|
| `user.registered` | `user-center-worker` | 欢迎积分 |
| `user.registered` | `user-center-notification-service` | Redis 欢迎消息 |
| `user.activity` | `user-center-worker` | 行为日志和排行榜 |

### 消费成功后何时提交 offset？

Consumer handler 返回成功后调用 `MarkMessage`。失败时不标记当前消息，并结束本轮 claim，让 Kafka 后续重新交付。

### 当前能否声称已接入 retry 和 DLQ？

不能。仓库里有 retry handler 原型和 DLQ topic 创建代码，但两个 consumer 入口没有组合 retry handler，也没有实际 DLQ consumer。不能宣称存在 1/5/30 秒延迟重试或完整死信闭环。

## 幂等与一致性

### 注册事件如何防重复？

- Redis 使用 processing/done 两阶段 key 防止同一 event 并发处理。
- 欢迎积分表使用业务唯一约束兜底。
- 欢迎消息使用 `SETNX`。
- 处理失败时清理 processing key，允许 Kafka 重投。

done key 默认保留 7 天，processing key 默认保留 5 分钟。这是有时间边界的去重，不等于永久 exactly-once。

### RankConsistencyCache 是否已接入？

没有。相关实验代码存在，但运行时使用的是 `RedisRankCache` 和用户行为处理 Lua 脚本。面试时只能讨论现有 ZSet/Lua 实现，不能声称版本 CAS 缓存已经上线。

## 配置与工程化

- Viper 合并默认值、YAML 和环境变量覆盖。
- 仅 `log.level` 与 `feature.*` 支持热更新；连接型依赖的配置修改后需要重启。
- Wire 生成主服务依赖图，worker 与 notification-service 使用显式初始化。
- Zap 输出结构化日志。
- Compose 同时运行 MySQL、Redis、Kafka 和三个 Go 服务。

## 可以诚实讨论的局限

- Outbox Relay 没有多实例抢占保护。
- Relay 失败后依赖下一轮轮询，没有专门退避算法。
- retry/DLQ 尚未接入 consumer 主链路。
- notification-service 当前只写 Redis 欢迎消息。
- 补偿任务入口没有实现真正扫描与修复。
- 没有账号级登录限流。
- 没有可引用的 QPS、P95/P99 或缓存命中率基准数据。

## 验证证据

M00 baseline 已执行过一次真实本地链路：

- Signup → MySQL → Outbox → Kafka → worker 欢迎积分 → notification Redis 欢迎消息。
- Login → JWT → Checkin → MySQL transaction → Outbox → Kafka → Redis 行为日志与日/月榜。
- `go build ./...`、`go test ./...`、`go vet ./...` 和 race tests 通过。

这些结果证明该次环境下链路可运行，但不代表生产容量或长期可靠性结论。
