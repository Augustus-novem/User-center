# M09 Learning Handoff

## 1. 解决的问题

在不让 HTTP 请求双写 MySQL/Elasticsearch 的前提下，为 published 笔记提供全文搜索；ES 故障时使用严格受限的 MySQL 查询维持基础可用性。

## 2. 完整调用链

```text
note publish/delete transaction
  → MySQL Outbox → Relay → Kafka
  → independent search worker group
  → SearchIndexService → Elasticsearch derived index

GET /search/notes
  → SearchHandler → SearchService
  → Elasticsearch with deadline
  → bounded MySQL fallback on unavailable/timeout
```

## 3. 核心代码

| Symbol | File | Responsibility |
|---|---|---|
| `SearchConfig` | `internal/config/type.go` | ES 地址、索引、group、timeout、fallback 边界 |
| `Elasticsearch` | `internal/integration/search/elasticsearch.go` | 隔离 ES REST request/response |
| `SearchIndexService` | `internal/service/search_index.go` | published/delete 投影与批量重建 |
| `SearchIndexHandler` | `internal/worker/search_index_handler.go` | Kafka event 解析与投影调用 |
| `SearchServiceImpl` | `internal/service/search.go` | request timeout、降级选择、输出模型 |
| `SearchRecentPublished` | `internal/repository/dao/note.go` | 有时间/字段/limit 边界的 DB fallback |
| `cmd/search-worker` | `cmd/search-worker/main.go` | 独立搜索消费组入口 |
| `cmd/search-reindex` | `cmd/search-reindex/main.go` | 从 MySQL 重建派生索引 |

## 4. Database

- 没有新表。
- 新索引：`idx_note_status_created(status, created_at, id)`。
- 删除笔记的 `status=deleted` 与 `note.deleted` Outbox 在同一事务。
- fallback 只读取 published、最近 30 天、最多 20 条，并只 select 搜索结果字段。

## 5. Elasticsearch

- Index：`community_notes`。
- 文档 ID：note ID。
- Mapping：long note/author ID，text title/content，epoch_millis created_at，keyword status。
- ES 是派生数据；`cmd/search-reindex` 提供可重建路径。

## 6. Kafka

- Topics：`note.published`、`note.deleted`。
- Group：`user-center-search-worker`，与 `user-center-worker` 隔离。
- 重复 publish 通过固定 document ID 覆盖；重复 delete 将 404 视为成功。
- 当前无 note update API，所以没有 `note.updated` producer。

## 7. Failure scenarios

1. ES 查询失败/超时：进入 300ms、30 天、20 条上限的 MySQL fallback，并返回 `degraded=true`。
2. ES 写入失败：search worker 返回错误，不提交该 partition offset；不影响其他 consumer group。
3. published/delete 跨 topic 乱序：published handler 会重新读取 MySQL；若已 deleted，则执行 ES delete。
4. 重复事件：固定 document ID 的 index/delete 是幂等投影。
5. ES 数据丢失：运行 reindex 命令从 MySQL published notes 重建。
6. DB fallback 也失败/超时：API 返回内部错误，不进行无界重试。

## 8. Trade-offs

- 标准库 REST client 减少 Go 依赖，但需要自行维护有限的 ES response mapping。
- 独立 search group 增加一个进程，但把 ES 故障域与核心 worker 隔离。
- LIKE fallback 不是全文检索替代品；时间窗口和 limit 是保护主库的必要约束。
- 当前按单条 REST 写索引，重建简单可恢复，但吞吐低于 Bulk API；现有数据量没有证据支持提前复杂化。

## 9. Interview questions

1. 为什么 Outbox 能避免 HTTP 双写的不一致？
2. 为什么 search worker 必须使用独立 consumer group？
3. published 与 deleted 分属不同 topic 时，如何避免乱序复活文档？
4. 固定 ES document ID 提供了什么幂等性，不能提供什么？
5. 为什么 fallback 必须同时限制时间、字段、limit 和 timeout？
6. `(status, created_at, id)` 索引对 LIKE 条件有哪些帮助和限制？
7. ES request timeout 与 DB fallback timeout 为什么要分开？
8. 为什么 ES 是派生数据仍需要持久化 volume？
9. reindex 过程中发生 publish/delete 会有什么竞态？
10. 何时才值得把单条重建改为 Bulk API？

## 10. 我自己编码的三个任务

1. 为 malformed `note.deleted` payload 增加表驱动测试，区分 JSON 错误和字段错误。
2. 为 fallback 增加 `_` 与反斜杠 LIKE 转义用例。
3. 写一个重建中途失败后从最后 note ID 恢复的设计草案，不修改当前命令。

> Agent 不提供本节答案，除非我完成尝试后请求 hint。

## Architecture Review

- [x] web → service → repository/contract → dao/integration。
- [x] service 不依赖 Gin、GORM、Viper、Wire 或 ES SDK。
- [x] ES request/response 被限制在 `internal/integration/search`。
- [x] IoC 集中在 `ioc/` 与 command composition root。
- [x] 删除事务边界在 NoteService 可见，缓存只在 commit 后失效。
- [x] ES 派生数据有明确重建命令。
- [x] 无 HTTP 双写、service locator、全局 mutable dependency 或无界 fallback。

## Verification

- build / unit test / vet / diff check：PASS。
- 真实 MySQL + Kafka + Elasticsearch 生命周期 E2E：PASS（13.54s）。
- Compose search-worker Linux 镜像 build/start：PASS；容器 restart count 0。
- E2E 验证 publish eventually visible、固定 ID 覆盖更新、搜索 API 非降级响应、delete eventually absent。
- ES timeout/down 与 fallback 时间/窗口/字段/limit 边界由单元测试覆盖。
- race：SKIPPED（本地 Windows 无受支持 C 编译器；按既定约束不修改系统环境）。
