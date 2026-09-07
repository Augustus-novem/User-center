# M02 Learning Handoff

## 1. 解决的问题

社区需要可发布的多图笔记，并且发布成功必须能可靠地进入现有 Outbox，而不是“先写库再发 Kafka”。

## 2. 完整调用链

```text
POST /notes
  → JWT middleware
  → web.NoteHandler.Publish
  → service.NoteServiceImpl.Publish
       校验 title/content/image_urls
       tx.InTx
         NoteRepository.Create
           dao.Insert notes
           dao.InsertImages note_images（sort_order = 请求数组下标）
         Publisher.Publish(note.published)
           EventOutboxRepository.Add  （同一 dbFromCtx 事务）
  → COMMIT
  → OutboxRelay 后续把 pending 行投递到 Kafka topic note.published
```

本阶段 worker / notification-service 不订阅 `note.published`。事件先可靠落库，消费者留给 Feed/Search。

```text
DELETE /notes/:id
  → FindByID
  → 非作者：ErrNoteForbidden
  → 作者：status=deleted
```

## 3. 核心代码

| Symbol | File | Input | Output | Responsibility |
|---|---|---|---|---|
| `NoteHandler` | `internal/web/note.go` | HTTP DTO | JSON | 登录用户、错误映射 |
| `NoteServiceImpl` | `internal/service/note.go` | author/title/images | domain.Note | 校验、事务编排、发 Outbox |
| `NoteRepositoryImpl` | `internal/repository/note.go` | domain.Note | notes + images | 聚合写入与读取 |
| `GORMNoteDAO` | `internal/repository/dao/note.go` | persistence | MySQL | notes/note_images |
| `NotePublishedEvent` | `internal/events/note_published.go` | note/author id | JSON payload | Outbox 事件形状 |
| `OutboxPublisher` | `internal/events/publisher.go` | topic/key/value | event_outbox 行 | 已有通用发布器 |

## 4. Database

Tables:

```text
notes(id, author_id, title, content, status, created_at, updated_at)
note_images(id, note_id, url, sort_order)
event_outbox 复用，不改表结构
```

Indexes:

```text
notes: idx_author_status_created(author_id, status, created_at, id)
note_images: uk_note_image_sort(note_id, sort_order)
```

Transactions: 发布必须在 `InTx` 内完成 note + images + outbox。任何一步失败都回滚，不会出现“笔记在、事件不在”或“事件在、笔记不在”。

## 5. Redis

Keys: 无

## 6. Kafka

Events: `note.published`
Topics: `note.published`（Relay 会创建；同时预创建 `.dlq` 名，但无 DLQ 消费）
Groups: 无新 group
Idempotency: 发布本身不是用户级幂等接口。事件 `event_id` 供未来消费者去重。

## 7. Failure scenarios

1. InsertImages 失败：事务回滚，notes 行不保留。
2. Outbox Publish 失败：事务回滚，笔记和图片都不存在。
3. Kafka 关闭：`NopPublisher`，只写笔记，不写 Outbox。
4. 非作者删除：不改 status。
5. 查询已软删笔记：对所有人返回不存在，避免泄漏删除态。

## 8. Trade-offs

- 软删除而不是物理删除：保留聚合根，删除接口仍然简单；图片行暂不级联删。
- 不在 M02 加消费者：没有 Feed/Search 时消费没有业务价值，只增加假实现。
- 图片只存 URL：对象存储是另一个系统，不在 community-v1 范围。
- 列表仍用 cursor：作者主页会持续插入新笔记。

## 9. Interview questions

1. 为什么 note 和 outbox 必须同一事务？
2. Outbox 解决了什么，还留下什么 at-least-once 窗口？
3. Kafka 关闭时为什么不写 Outbox？
4. 为什么本阶段不让 worker 消费 `note.published`？
5. sort_order 为什么按请求数组下标而不是客户端传入的随意数字？
6. 软删除后详情为什么不返回 410/deleted？
7. 非作者删除为什么要先 Find 再拒绝，而不是 UPDATE WHERE author_id=? 一行？
8. Relay 按 topic 字段投递，新增事件为什么不用改 Relay 代码？
9. 如果 Insert notes 成功、进程在 InsertImages 前崩溃，用户会看到什么？
10. 未来 Push Feed 为什么适合用 author_id 做 Kafka key？

## 10. 我自己编码的三个任务

1. 给 `POST /notes` 加上“同一作者 1 秒内相同 title 视为重复发布”的防护，并说明为什么这不能替代 Outbox。
2. 写测试：Create 成功后 InsertImages 返回错误，断言 repository 上层事务不会留下 notes 行（可用现有 mem store 思路）。
3. 如果要支持编辑笔记，`note.published` 还够不够？先设计 `note.updated` 的 payload 和事务边界，不要直接改代码。

> Agent 不提供本节答案，除非我完成尝试后请求 hint。

## Architecture Review

- [x] web → service → repository → dao。
- [x] 异步：service → outbox → Kafka；本阶段无 consumer handler。
- [x] service 不依赖 Gin/GORM/Wire。
- [x] 发布事务边界在 `NoteServiceImpl.Publish` 可见。
- [x] HTTP DTO 与 GORM model 分离。
- [x] IoC 仍在 Wire；无新万能包、无目录膨胀。
- [x] 复用现有 Publisher/Transaction，不平行造第二套 Outbox。
