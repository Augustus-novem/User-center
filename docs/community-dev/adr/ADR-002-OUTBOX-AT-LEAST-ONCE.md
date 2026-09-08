# ADR-002：Outbox + At-least-once + Idempotent Consumer

**Status:** Accepted

业务数据和 Outbox Event 在同一个 MySQL 本地事务提交，Relay 投递 Kafka，消费者按照消息可能重复设计。

Outbox 不能天然解决“Kafka 已发布成功，但 Relay 在标记 sent 前崩溃”，所以不宣称 exactly-once。消费者使用 event_id / 存储唯一约束幂等。

Redis 两阶段消费者幂等采用 `ACQUIRED / DONE / BUSY` 三态。processing key 保存随机 owner token；只有 owner 可通过 Lua `MarkDone` 或 `ClearInFlight`。`DONE` 可提交 offset，`BUSY` 返回错误且不得提交，业务失败使用独立 bounded context 清理 lease。

永久非法 JSON/event 先同步写入 `${topic}.dlq`，DLQ ack 后才允许提交原 offset。Redis/MySQL/Elasticsearch/timeout 属于 transient failure，保持原 offset 未提交以等待 redelivery。
