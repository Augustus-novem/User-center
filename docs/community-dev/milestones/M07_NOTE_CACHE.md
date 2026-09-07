# M07 Note 多级缓存

## 目标

为笔记详情读取增加有界多级缓存，并明确缓存穿透、热点回源和一致性边界。

## 分支

`feat/community-m07-note-cache`

## 提交顺序

1. Redis 正缓存。
2. Negative cache。
3. TTL jitter。
4. singleflight。
5. 短生命周期 local cache。

## 读取链路

```text
GET /notes/:id
  → NoteHandler
  → NoteService.Get
  → CachedNoteRepository.FindByID
      → local cache
      → Redis note:detail:{note_id}
      → singleflight（按 note_id、仅包住回源区间）
          → 二次检查 local / Redis
          → NoteRepositoryImpl
          → NoteDAO.FindByID + ListImages
          → MySQL
          → Redis / local 回填
```

MySQL 始终是真实数据源。Redis miss 或故障时回源；Redis 读写失败只记录 warning，不让缓存成为详情接口的硬依赖。

## 缓存参数

| 层级 | 内容 | TTL / 容量 |
|---|---|---|
| local | 正条目与 not-found tombstone | 1 秒；单进程最多 1024 条 |
| Redis positive | 完整 `domain.Note` JSON | 10 分钟 ±10% jitter |
| Redis negative | `{"not_found":true}` | 1 分钟 ±10% jitter |

local cache 在容量满时淘汰最早到期条目；读取返回图片切片深拷贝，避免调用方修改共享对象。

## singleflight 边界

- 合并键是十进制 `note_id`，不同笔记互不阻塞。
- 只合并当前进程的并发 miss，不是分布式锁。
- singleflight 内二次检查缓存，避免前一个请求已回填后仍访问 MySQL。
- 等待者可以由自己的 context 提前返回；实际 leader 完成后结果才会从 group 移除。

## 创建与删除一致性

- Publish 的 MySQL Note 与 `note.published` Outbox 位于同一事务。
- 新自增 ID 的缓存失效必须在事务提交后执行。否则并发读可能在“事务内提前失效”和“提交”之间重新写入 negative cache。
- SoftDelete 成功后删除当前进程 local entry 与 Redis key。
- 当前没有 Note update API，因此 M07 没有声称或测试不存在的 update 路径。未来新增 update 时必须复用提交后失效规则。

这不是强一致缓存：

- 多实例中，其他进程的 local entry 最多陈旧约 1 秒。
- Redis invalidation 失败时会记录 warning；已有正缓存最坏保留到 jitter 后的 TTL 上界约 11 分钟。
- negative cache 的跨进程 local 陈旧窗口约 1 秒；Publish 提交后会再次删除 Redis key。

## 验证

- Unit：Redis hit/miss、negative entry、TTL jitter 边界、local TTL/容量/深拷贝。
- Repository：cache miss 回源、negative hit 不回源、64 并发热点 miss、Redis down 降级、delete/local invalidation。
- Service：Publish 提交后才 invalidate 新 ID。
- Redis integration：真实 Redis round trip、positive/negative TTL、delete 后 miss。
- Comparison：singleflight ON/OFF 的 origin load、DB query count 与 wall-clock latency。

对照原始结果见 `docs/community-dev/benchmarks/2026-09-08-m07-singleflight.md`。

## DoD 结果

- `go build ./...`：PASS。
- `go test ./...`：PASS。
- `go vet ./...`：PASS。
- `git diff --check`：PASS。
- 真实 Redis E2E：PASS。
- singleflight 并发回源对照：PASS。
- `go test -race ./...`：`SKIPPED - local Windows environment has no supported C compiler.`

Race skip 是经用户批准的本地环境限制，不是代码失败。不得为了 milestone 自动安装系统级开发工具。
