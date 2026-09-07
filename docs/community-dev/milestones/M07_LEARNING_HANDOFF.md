# M07 Learning Handoff

## 1. 解决的问题

热点 Note 详情在缓存冷启动或过期时会同时回源 MySQL；不存在的 ID 会反复穿透。M07 用 Redis、negative cache、TTL jitter、singleflight 和短 TTL local cache 分层控制这些成本，同时保留 MySQL 真相。

## 2. 完整调用链

```text
GET /notes/:id
  → Handler → Service → CachedNoteRepository
  → local(1s) → Redis(note:detail:{id})
  → singleflight(id) → 二次 cache check
  → NoteRepositoryImpl → NoteDAO → MySQL
  → Redis + local 回填
```

Publish 在事务提交后失效新 ID；SoftDelete 在数据库成功后失效 Redis 和当前进程 local。

## 3. 核心代码

| Symbol | File | Responsibility |
|---|---|---|
| `RedisNoteCache` | `internal/repository/cache/note.go` | Redis JSON、negative tombstone、TTL jitter、key |
| `MemoryNoteLocalCache` | `internal/repository/cache/note_local.go` | 1 秒有界进程缓存、深拷贝、淘汰 |
| `CachedNoteRepository` | `internal/repository/note_cache.go` | cache-aside、fail-open、singleflight、失效 |
| `NoteCacheInvalidator` | `internal/repository/note.go` | 提交后失效的最小可选能力 |
| `NoteServiceImpl.Publish` | `internal/service/note.go` | 事务完成后触发新 ID 失效 |
| `TestNoteCacheSingleflightComparison` | `internal/repository/note_cache_comparison_test.go` | ON/OFF 并发回源对照 |

## 4. Database

无新表、索引或迁移。一次详情 origin load 仍包含 `notes` 行查询和 `note_images` 查询。MySQL 是唯一真相源。

## 5. Redis

- Key：`note:detail:{note_id}`。
- Positive：完整 Note JSON，基准 TTL 10 分钟，±10% jitter。
- Negative：`{"not_found":true}`，基准 TTL 1 分钟，±10% jitter。
- Redis miss/故障都允许回源；故障会记录 warning。

## 6. Kafka

无新 topic/group/event。`note.published` 与 Outbox 行为不变；缓存只影响详情读取与 Publish 提交后的 ID 失效。

## 7. Failure scenarios

1. Redis miss：进入 singleflight 并回源 MySQL。
2. Redis down：记录 warning，详情仍从 MySQL 返回。
3. 不存在或已删除：写 negative cache，后续短路。
4. 同一热点并发 miss：同进程只允许一个 leader 回源。
5. Publish rollback：不执行提交后失效；数据库也没有新 Note。
6. Publish commit：提交后再次失效，避免自增 ID 旧 tombstone。
7. SoftDelete：当前实例 local 与 Redis 失效；其他实例 local 最多陈旧约 1 秒。
8. Redis invalidation 失败：日志可见，旧正缓存受 Redis TTL 上界约束。

## 8. Trade-offs

- local cache 降低 Redis 调用，但引入最多约 1 秒的多实例陈旧窗口。
- singleflight 不是分布式锁；每个实例仍可能各自回源一次。
- negative cache 缓解穿透，但自增 ID 创建必须严格在 commit 后再次失效。
- 缓存错误 fail-open 保证可用性，却可能增加 MySQL 压力。
- TTL 与容量目前是代码常量；若以后需要按环境调优，应通过配置注入，不能在业务层读取 Viper。

## 9. Interview questions

1. 为什么 singleflight 要放在第一次 Redis miss 之后？
2. 为什么进入 singleflight 后还要二次检查缓存？
3. negative cache 与普通 miss 如何区分？
4. 为什么 new Note ID 的失效必须在事务提交后？
5. local cache 为什么必须有容量上限？
6. 多实例 local cache 的 stale window 如何产生？
7. Redis 故障时为何选择 fail-open？
8. TTL jitter 解决的是哪个时间维度的热点问题？
9. 为什么 ON/OFF 对照中的 DB query count 比 elapsed 更稳定？
10. 为什么这份对照不能宣称生产 P99？

## 10. 我自己编码的三个任务

1. 给 local cache 增加可注入的淘汰策略，并写出容量边界测试。
2. 写一个多 note_id 并发测试，证明 singleflight 不会把不同 key 串行化。
3. 设计 Note update API 的提交后失效测试，只提交测试草案，不实现 API。

> Agent 不提供本节答案，除非我完成尝试后请求 hint。

## Architecture Review

- [x] web/service 不直接读取 Redis。
- [x] cache 实现位于 repository/cache，注入在 Wire composition root。
- [x] singleflight 是 repository decorator 的横切能力，没有 global mutable state。
- [x] MySQL 是真相源，没有把缓存模型泄漏到 domain。
- [x] 没有新增搜索、热榜、通知或分布式锁。
