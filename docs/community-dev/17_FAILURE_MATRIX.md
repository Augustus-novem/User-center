# Community V1 Failure Matrix

| Failure | Current behavior | Data safety | Recovery / evidence | Known limit |
|---|---|---|---|---|
| Kafka unavailable during Relay send | Outbox stays pending; attempts/last_error update; later dispatch retries | business row and event remain in MySQL | `TestOutboxRelay_BacklogRecoversAfterKafkaFailure` | no exponential Relay backoff |
| Kafka ack succeeds, Outbox mark fails | row remains pending and may be sent again | at-least-once; consumer must dedup | `TestOutboxRelay_AcknowledgedMessageCanBeRedelivered` | duplicates are expected |
| Two Relay instances scan one row | both may send; one may fail `MarkPublished` after the other wins | consumer idempotency protects supported effects | observed by first shared M11 benchmark (`event outbox not found`) | no claim/lease; not fixed in M11 |
| Consumer handler fails | message not marked; claim returns error | Kafka offset remains eligible for redelivery | `TestConsumerGroupHandler_RedeliversAfterSessionRestart` | command loop uses fixed 1s restart delay |
| Duplicate Kafka event | Redis/DB/ES mechanism suppresses duplicate side effect | varies by consumer, catalogued per event | worker tests + M05/M08/M10 E2E | not a global exactly-once guarantee |
| Redis down on Note detail | cache errors are logged; request loads MySQL | MySQL remains truth | `TestCachedNoteRepository_RedisFailureFallsBackToOrigin` | DB load rises; no circuit breaker |
| Concurrent Note cache miss | singleflight coalesces per-process origin loads | response correctness unchanged | M07 ON/OFF: 128→2 DB queries for 64 callers | not cross-process coalescing |
| Feed inbox write fails mid-fanout | handler fails, clears in-flight marker; replay repeats idempotent ZADD | completed batches remain, missing batches recover | Feed service + note handler failure tests | partial progress is visible until retry |
| Elasticsearch query down/timeout | 800ms ES deadline then 300ms, 30-day, max-20 MySQL fallback | MySQL truth unchanged; response says degraded | Search service tests | LIKE fallback is intentionally limited |
| Elasticsearch indexing worker down | core write succeeds; ES becomes stale | Outbox/Kafka retains event according to normal retention | M09 lifecycle + reindex command | consumer lag metrics not exported |
| MySQL down | write/read paths fail; Redis-only derived data cannot accept truth writes | no false success intended | repository/service error tests | no HA/failover in Compose |
| notification-service down | core action succeeds and event remains in Kafka/Outbox path | notification eventually catches up after restart | M10 Kafka E2E + consumer semantics | no external push channel |

## Retry/DLQ statement

`.dlq` topic names and a `RetryableHandler` prototype exist, but no runtime command composes that handler. Kafka message timestamps do not implement broker-side delayed delivery. Therefore community-v1 does not claim functional 1/5/30-second retries or DLQ routing.
