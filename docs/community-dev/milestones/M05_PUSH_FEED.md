# M05 Push Feed：Kafka Fan-out

## 目标
利用 `note.published` + Kafka，把内容扇出到粉丝 Redis Inbox。

## 分支
`feat/community-m05-push-feed`

## Redis

```text
feed:inbox:{user_id}
ZSET
member = note_id
score  = publish timestamp
```

若 timestamp 冲突影响排序，必须设计稳定 tie-break。

## Flow

```text
note published → outbox → relay → Kafka → feed worker → followers → ZADD inbox
```

## 幂等
按 at-least-once 处理，复用现有 event_id 消费幂等；同时理解 ZADD 同 member 的行为。

## 必测
普通 fanout、duplicate event、consumer 重启、Redis 失败、follower=0、大 follower 批处理。

## Benchmark
与 M04 Pull 对比 publish cost、read latency、write amplification。
