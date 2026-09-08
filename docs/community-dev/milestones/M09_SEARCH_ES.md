# M09 Elasticsearch 搜索 + 有界降级

## 目标
引入项目唯一一个新的重量级基础设施。

## 分支
`feat/community-m09-search`

## Index
`note_id`、`author_id`、`title`、`content`、`created_at`、`status`。

## Index Flow
禁止 HTTP 双写 MySQL+ES：

```text
note.published / note.updated / note.deleted → Kafka → search index worker → Elasticsearch
```

ES 是派生索引，可重建。

## API
`GET /search/notes?q=...`

## Reliability
必须有 request timeout 和 bounded fallback。DB fallback 限制时间范围/字段/limit，并有 DB timeout。不能让 ES down 导致全库 `%keyword%` 扫描拖垮主库。

## 必测
index eventually visible、update/delete、ES timeout/down、fallback boundary。

## 实现结果

### 调用链

```text
POST /notes
  → NoteService MySQL transaction
  → notes + event_outbox(note.published)
  → Relay → Kafka
  → search-worker (user-center-search-worker)
  → SearchIndexService → Elasticsearch PUT /community_notes/_doc/{note_id}

DELETE /notes/{id}
  → notes.status=deleted + event_outbox(note.deleted) in one transaction
  → Relay → Kafka → search-worker
  → Elasticsearch DELETE /community_notes/_doc/{note_id}

GET /search/notes?q=...
  → SearchService (800ms ES deadline)
  → Elasticsearch
  → on disabled/error/timeout: MySQL fallback (300ms deadline)
     status=published + created_at within 30 days + fields/limit bounded
```

### 派生索引与重建

- Index：`community_notes`。
- 字段：`note_id`、`author_id`、`title`、`content`、`created_at`、`status`。
- 文档 ID 固定为 note ID；Kafka 重复投递执行覆盖写/幂等删除。
- `go run ./cmd/search-reindex --config=config/worker.yaml` 按 note ID 升序、每批 200 条重建 published 笔记。
- ES 不是业务真相源；MySQL 与 Outbox 是真相源。

### 有界降级

- ES request timeout：默认 800ms。
- DB fallback timeout：默认 300ms。
- fallback window：最近 30 天。
- limit：默认 10，上限 20。
- DB 只 Select 搜索返回字段，并通过 `idx_note_status_created(status, created_at, id)` 先约束状态与时间窗口。
- 查询结果以 `degraded=true` 明确告知调用方正在使用 MySQL 降级。

### 当前边界

- 当前项目没有笔记更新 API，因此没有凭空新增 `note.updated` producer。ES 适配器验证了同 document ID 覆盖更新语义；未来只有在业务更新事务真实存在时，才应在同一事务写 `note.updated` Outbox。
- search worker 使用独立 consumer group；ES 故障不会阻塞现有 `user-center-worker` 的积分、行为、Feed 和热榜消费。
- Compose 的 Elasticsearch 关闭 security 仅用于本地开发，不是生产部署模板。

## 验证结果（2026-09-08）

- `go build ./...`：PASS。
- `go test ./...`：PASS。
- `go vet ./...`：PASS。
- `git diff --check`：PASS。
- `go test -tags=e2e -run '^TestSearch_NoteLifecycleThroughKafkaE2E$' -count=1 -v .`：PASS（13.54s）。
- E2E 随机 Elasticsearch index 已在测试 cleanup 中删除并实查无残留。
- `docker compose up -d --build search-worker`：PASS；容器 `running`、restart count 0，日志确认使用 group `user-center-search-worker`。
- `go test -race ./...`：SKIPPED，沿用已记录的本地 Windows 无受支持 C 编译器环境限制；未安装或调整系统工具链。
