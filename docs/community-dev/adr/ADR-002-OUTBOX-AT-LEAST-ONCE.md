# ADR-002：Outbox + At-least-once + Idempotent Consumer

**Status:** Accepted

业务数据和 Outbox Event 在同一个 MySQL 本地事务提交，Relay 投递 Kafka，消费者按照消息可能重复设计。

Outbox 不能天然解决“Kafka 已发布成功，但 Relay 在标记 sent 前崩溃”，所以不宣称 exactly-once。消费者使用 event_id / 存储唯一约束幂等。
