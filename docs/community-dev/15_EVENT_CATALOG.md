# Community V1 Event Catalog

所有事件都含 UUID `event_id`、事件名 `type` 和毫秒时间戳 `occurred_at`。业务写入与 `event_outbox` insert 位于同一 MySQL 事务，Relay 发布成功后标记 Outbox 为 `published`。

| Topic | Extra payload | Producer | Consumer / group | Side effect | Idempotency |
|---|---|---|---|---|---|
| `user.registered` | `user_id,email` | UserService | worker / `user-center-worker` | welcome point record | DB `(biz_type,biz_id,user_id)` unique |
| `user.registered` | `user_id,email` | UserService | notification / `user-center-notification-service` | Redis welcome message | Redis two-phase event dedup + `SETNX` |
| `user.activity` | `user_id,action,biz_id,points` | SignInService | worker / `user-center-worker` | activity log + daily/monthly rank | Redis Lua `worker:event:done:{event_id}` |
| `user.followed` | `follower_id,followee_id` | FollowService | notification / `user-center-notification-service` | MySQL follow notification | `notifications.event_id` unique |
| `note.published` | `note_id,author_id` | NoteService | worker / `user-center-worker` | Feed fanout + hot score | Feed event Redis dedup; hot rank Lua event dedup |
| `note.published` | `note_id,author_id` | NoteService | search / `user-center-search-worker` | ES document upsert | fixed ES document ID |
| `note.deleted` | `note_id,author_id` | NoteService | search / `user-center-search-worker` | ES document delete | repeated delete/404 succeeds |
| `note.liked` | `note_id,user_id` | EngagementService | worker / `user-center-worker` | hot score | hot rank Lua event dedup |
| `note.liked` | `note_id,user_id` | EngagementService | notification / `user-center-notification-service` | MySQL like notification | `notifications.event_id` unique |
| `comment.created` | `comment_id,note_id,user_id` | EngagementService | worker / `user-center-worker` | hot score | hot rank Lua event dedup |
| `comment.created` | `comment_id,note_id,user_id` | EngagementService | notification / `user-center-notification-service` | MySQL comment notification | `notifications.event_id` unique |

## Topic provisioning

`EnsureKafkaTopics` ensures every source topic above and its `.dlq`-suffixed name exist for local development. Every runtime consumer publishes permanently invalid JSON/events to `${topic}.dlq`; the source offset is marked only after the synchronous DLQ publish succeeds. Transient infrastructure errors and `BUSY` deduplication leases are not sent to DLQ and are not marked.

## Ordering

- Local Compose topics use one partition, but correctness still must not assume cross-topic order.
- Search re-reads MySQL on `note.published`; if the note is already deleted, it deletes the ES document instead of reviving it.
- Consumers mark a message only after its handler returns success. Handler failure ends the claim so an uncommitted offset can be redelivered after a new session.
