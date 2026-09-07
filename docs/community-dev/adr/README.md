# Architecture Decision Records

ADR 用来记录“为什么这样做”，防止架构选择只存在于聊天记录。

| ID | Title |
|---|---|
| [ADR-001](ADR-001-KEEP-KAFKA.md) | 继续使用 Kafka，不换成 RabbitMQ |
| [ADR-002](ADR-002-OUTBOX-AT-LEAST-ONCE.md) | Outbox at-least-once |
| [ADR-003](ADR-003-HYBRID-FEED.md) | Hybrid Feed |
| [ADR-004](ADR-004-CACHE-CONSISTENCY.md) | 缓存一致性 |
| [ADR-005](ADR-005-SEARCH_DEGRADATION.md) | 搜索降级 |
| [ADR-006](ADR-006-GO-INTERFACE-STRATEGY.md) | Go interface 策略 |
| [ADR-007](ADR-007-IOC-AND-CROSS-CUTTING.md) | IoC 与横切 |
| [ADR-008](ADR-008-INBOX-SCORE-NOTE-ID.md) | Push Feed Inbox 用 note_id 做 ZSET score |

格式：

```text
Status
Context
Decision
Alternatives
Consequences
Validation
```
