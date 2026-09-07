# M07 Note 多级缓存

## 目标
优化热点笔记详情读取，并明确缓存一致性。

## 分支
`feat/community-m07-note-cache`

## 分步提交
1. Redis cache。
2. Negative cache。
3. TTL jitter。
4. singleflight。
5. 短 TTL local cache（前四步稳定后）。

## Flow

```text
local cache → Redis → singleflight → MySQL
```

singleflight 围绕真正回源区间。

## 更新/删除
定义 Redis invalidation、本地 cache invalidation 或短 TTL、可接受 stale window。不要声称“强一致多级缓存”。

## 必测
hit/miss、不存在 Note、热点并发 miss、Redis down、update/delete。

## Benchmark
singleflight ON/OFF 比较 DB query count 和 latency。
