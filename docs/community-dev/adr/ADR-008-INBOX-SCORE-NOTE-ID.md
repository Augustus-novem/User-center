# ADR-008：Push Feed Inbox 使用 note_id 作为 ZSET score

**Status:** Accepted

**Context:** M05 将 `note.published` 扇出到 Redis `feed:inbox:{user_id}`。里程碑原文曾用 publish timestamp 做 score，同毫秒需要额外 tie-break，且 Redis ZSET score 是 float64，不适合塞复合时间戳+id。

**Decision:** member 与 score 都使用 `note_id`。

当前笔记在 INSERT 时由 MySQL 自增分配 ID，发布即创建，不存在“先草稿后改 ID 再发布”。因此 `note_id` 单调递增，足以作为严格稳定的发布顺序代理。

**Alternatives:**

- score=occurred_at，并列再用 member tie-break：同毫秒多篇笔记顺序不稳定，除非再引入第二维。
- 把 timestamp 和 id 编码进 float64：精度不够，实现复杂。

**Consequences:**

- 排序与 cursor 都按 note_id DESC，实现简单，ZADD 同 member 天然幂等。
- 若未来支持草稿转发布、或 ID 不再单调，必须另写 ADR 改 score 模型。
- 本决策只约束 M05 Inbox，不改变 Outbox `occurred_at` 或其它事件时间语义。

**Validation:** M05 unit tests cover trim、duplicate ZADD、stable note_id cursor；E2E 重放同一 event_id 不产生重复 Feed。
