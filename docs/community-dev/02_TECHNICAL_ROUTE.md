# 02 技术路线总表

| Milestone | 核心功能 | 技术 | 学习点 |
|---|---|---|---|
| M00 | 基线冻结 | Git / Tests | 阅读现有系统、可回滚基线 |
| M01 | Follow | MySQL / GORM | 唯一索引、幂等、cursor |
| M02 | Note | MySQL / Outbox | 事务、事件、图片子表 |
| M03 | Like/Comment | MySQL | 唯一约束、业务正确性 |
| M04 | Pull Feed | SQL / Cursor | baseline、稳定分页、索引 |
| M05 | Push Feed | Kafka / Redis ZSET | Fan-out、最终一致、幂等 |
| M06 | Hybrid Feed | Push + Pull | 写放大、merge、trade-off |
| M07 | Note Cache | Redis / Local / singleflight | 穿透、击穿、雪崩、一致性 |
| M08 | Hot Ranking | Redis ZSET | 分钟桶、滑动窗口、快照 |
| M09 | Search | Elasticsearch / Kafka | 异步索引、timeout、降级 |
| M10 | Notification | Kafka / MySQL | 事件驱动业务化 |
| M11 | Reliability | Test/Benchmark/Failure | 故障恢复、性能证据 |

## 技术栈策略

保留：Go、Gin、GORM、MySQL、Redis、Kafka/Sarama、Wire、Viper、Zap、Docker Compose。

新增仅在对应 milestone：Go `singleflight`、Elasticsearch。

暂缓：Prometheus/Grafana、K8s。AI/Agent 是未来独立项目，不属于 `community-v1`。

## Reliable Event

```text
MySQL transaction:
    write business row
    write outbox row
commit
    |
relay
    |
Kafka (at-least-once)
    |
idempotent consumer
```

Outbox 不等于 exactly-once。Relay 在“Kafka 成功、outbox 尚未 mark sent”时崩溃可能重复投递，因此消费者必须幂等。

## Feed 演进

```text
M04 Pull baseline
   ↓
M05 Push inbox
   ↓
M06 Hybrid
```

## Cache 演进

```text
MySQL only → Redis → negative cache → TTL jitter → singleflight → short-lived local cache
```
