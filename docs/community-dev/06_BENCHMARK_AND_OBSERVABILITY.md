# 06 Benchmark 与可观测性

## 性能数据必须记录

机器环境、commit hash、数据规模、并发数、请求数/持续时间、命令、原始输出、是否 warm cache。

## Note Detail 对照

MySQL only / Redis hit / Redis miss / SingleFlight ON-OFF / Local cache hit。

指标：QPS、P50/P95/P99、DB query count。

## Feed 对照

follower 规模建议 100、1k、10k、50k（机器允许时）。比较 Pull/Push/Hybrid：publish latency、read latency、fanout write count。

## Outbox/Kafka

outbox pending、relay batch、publish latency、consume latency、duplicate/retry count。

## Search

ES latency、fallback rate、DB fallback latency。

## 日志字段

`request_id`、`event_id`、`event_type`、`user_id`、`note_id`、`consumer_group`、`topic`、`partition`、`offset`、`latency_ms`、`error`。

Benchmark 文件放 `docs/community-dev/benchmarks/YYYY-MM-DD-<name>.md`。
