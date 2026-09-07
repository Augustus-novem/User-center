# M05 Push Feed：Kafka Fan-out

## 目标
利用 `note.published` + Kafka，把内容扇出到粉丝 Redis Inbox。读路径从 M04 Pull JOIN 演进为 Inbox candidate + MySQL 过滤。

## 分支
`feat/community-m05-push-feed`

## 不做

- 不实现 celebrity / `fanout_threshold`（M06）。
- 不实现 Elasticsearch、热榜、通知。
- 不编造与 M04 对比的 P95 / QPS。

## Redis

```text
feed:inbox:{user_id}
ZSET
member = note_id
score  = note_id
```

**顺序代理：** 当前 `notes.id` 为 MySQL 自增，笔记在 INSERT 时获得 ID，发布即创建。用 `note_id` 同时做 member 和 score，得到严格稳定的发布顺序，避免 timestamp 并列，也避免把毫秒时间戳与 id 编码进 Redis double。这是 M05 的局部约束，不是全局事件时间模型。

裁剪：ZADD 后若 `ZCARD > feed.inbox_max_items`，用 Lua `ZREMRANGEBYRANK` 删掉最低分（更旧的 note_id）。Lua 保证 trim 与写入原子。

## 配置

```text
feed.fanout_batch_size   # 粉丝分页大小，默认 200
feed.inbox_max_items     # Inbox 上限，默认 500
```

## Flow

```text
note.published
  → event_outbox（已有发布事务）
  → Outbox Relay
  → Kafka topic note.published
  → worker group user-center-worker
  → TryBegin(event_id)
  → page followers by (created_at,id) cursor，每批 pipeline Lua ZADD+trim
  → 全部 batch 成功后 MarkDone
  → 任一 batch 失败：ClearInFlight，handler 返回 error，Kafka 不提交 offset
```

重试会再次扫全部粉丝。`ZADD` 同一 member 只更新 score，Feed 不重复。

读：

```text
GET /feed/following
  → ZREVRANGEBYSCORE inbox（score=note_id DESC）
  → FindByIDs
  → 丢掉 deleted/unpublished/missing
  → 丢掉作者已不在 following 中的 stale candidate
  → 不足一页则继续扫 inbox，不因脏 candidate 报错
cursor = 上一页最后一条有效笔记的 note_id
```

## 幂等

复用 `consumer:event:done|processing:{namespace}:{event_id}`。namespace = `worker:note_published`。

## 必测
0 follower、单 follower、多 batch、duplicate event、部分 batch 失败可重试、inbox trim、稳定 cursor、deleted note、unfollow stale candidate。
Docker E2E：B follow A → publish → outbox → Kafka → worker → inbox B → GET feed；重放同一 event_id 不重复。
