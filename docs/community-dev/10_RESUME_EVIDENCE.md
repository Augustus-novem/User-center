# 10 简历证据管理

只有已完成并有证据的内容才能写入简历。

每个亮点应能追溯到：**设计文档 + commit + test + benchmark/failure injection**。

## Hybrid Feed

需要保留 Pull/Push/Hybrid 数据、follower 规模、threshold、机器环境、测试脚本 commit。没测到数字就不写数字。

## Cache

对照 without cache / Redis hit / singleflight off-on。

## Outbox

证明 Kafka down 时业务 commit、Outbox pending、恢复后 Relay 投递、duplicate event 不重复副作用。

## 最终第一项目优先亮点

1. Hybrid Feed trade-off。
2. Multi-level Cache + SingleFlight。
3. Outbox + At-least-once + Idempotent Consumer。
4. Sliding Window Hot Ranking。
5. ES 异步索引 + bounded degradation。
6. Benchmark / failure injection。

CRUD API 数量不是第一亮点。
