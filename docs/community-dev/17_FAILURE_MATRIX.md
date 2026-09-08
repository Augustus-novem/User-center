# Community V1 Failure Matrix

| Failure | Current behavior | Data safety | Recovery / evidence | Known limit |
|---|---|---|---|---|
| Kafka unavailable during Relay send | Outbox stays pending; attempts/last_error update; later dispatch retries | business row and event remain in MySQL | `TestOutboxRelay_BacklogRecoversAfterKafkaFailure` | no exponential Relay backoff |
| Kafka ack succeeds, Outbox mark fails | row remains pending and may be sent again | at-least-once; consumer must dedup | `TestOutboxRelay_AcknowledgedMessageCanBeRedelivered` | duplicates are expected |
| Two Relay instances scan one row | both may send; one may fail `MarkPublished` after the other wins | consumer idempotency protects supported effects | observed by first shared M11 benchmark (`event outbox not found`) | no claim/lease; not fixed in M11 |
| Consumer handler transiently fails or lease is BUSY | message not marked; claim returns error | Kafka offset remains eligible for redelivery | `TestConsumerGroupHandler_RedeliversAfterSessionRestart`, BUSY tests | command loop uses fixed 1s restart delay |
| Consumer receives permanently invalid JSON/event | synchronously publish `${topic}.dlq`; mark source only after DLQ ack | poison message cannot silently disappear when DLQ is unavailable | ConsumerGroup unit tests + poison-message E2E | no automated DLQ consumer |
| Duplicate Kafka event | Redis/DB/ES mechanism suppresses duplicate side effect | varies by consumer, catalogued per event | worker tests + M05/M08/M10 E2E | not a global exactly-once guarantee |
| Redis down on Note detail | cache errors are logged; request loads MySQL | MySQL remains truth | `TestCachedNoteRepository_RedisFailureFallsBackToOrigin` | DB load rises; no circuit breaker |
| Concurrent Note cache miss | singleflight coalesces per-process origin loads | response correctness unchanged | M07 ON/OFF: 128→2 DB queries for 64 callers | not cross-process coalescing |
| Feed inbox write fails mid-fanout | handler uses an independently bounded context to clear only its owner-token lease; replay repeats idempotent ZADD | completed batches remain, missing batches recover | Feed service + note handler failure tests | partial progress is visible until retry |
| Elasticsearch query down/timeout | 800ms ES deadline then 300ms, 30-day, max-20 MySQL fallback | MySQL truth unchanged; response says degraded | Search service tests | LIKE fallback is intentionally limited |
| Elasticsearch indexing worker down | core write succeeds; ES becomes stale | Outbox/Kafka retains event; rebuild writes a temp physical index and atomically swaps the alias | M09 lifecycle + alias rebuild E2E | consumer lag metrics not exported |
| MySQL down | write/read paths fail; Redis-only derived data cannot accept truth writes | no false success intended | repository/service error tests | no HA/failover in Compose |
| notification-service down | core action succeeds and event remains in Kafka/Outbox path | notification eventually catches up after restart | M10 Kafka E2E + consumer semantics | no external push channel |

## Retry/DLQ statement

There is no retry topic or timestamp-based delayed delivery. Permanent errors use DLQ routing; transient errors rely on Kafka redelivery from an uncommitted source offset.
