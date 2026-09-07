# ADR-001：继续使用 Kafka

**Status:** Accepted

当前 User-center 已有 Kafka/Sarama、Consumer Group、Outbox Relay、消费者幂等和 worker。社区 Feed、Search、Notification 继续复用这条链路，不切换 RabbitMQ。

原因：减少无意义迁移，把学习重点放在事件一致性、幂等和业务设计。
