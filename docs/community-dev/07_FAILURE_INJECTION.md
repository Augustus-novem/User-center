# 07 故障注入计划

项目竞争力来自“故障时发生什么”，不只是 happy path。

## Outbox

模拟 DB commit 成功但 Kafka 不可用；Kafka 成功但 Relay 未 mark sent 就退出；Relay 重启。验证业务记录、pending 恢复、重复投递与 consumer 幂等。

## Feed Worker

重复 `note.published`、Redis 不可用、worker 中途退出。验证 MarkMessage 时机和幂等。

## Cache

Redis down、热点 key 同时失效、DB 延迟。验证 singleflight、fallback、stale window。

## Elasticsearch

connection refused、timeout、连续错误。验证有界 fallback，不能全库 `%keyword%` 拖垮 MySQL。

## Kafka

down、consumer restart、rebalance。记录主流程影响、backlog 恢复、duplicate handling。
