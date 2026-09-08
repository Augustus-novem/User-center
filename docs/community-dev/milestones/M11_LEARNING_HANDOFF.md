# M11 Learning Handoff

## 1. 解决的问题

停止扩展业务，把 M01–M10 的关键一致性、故障恢复和性能 trade-off 变成可执行测试、可复现 benchmark 与集中式目录，形成不夸大的 `community-v1` 交付证据。

## 2. Reliability chain

```text
MySQL business transaction + Outbox
  → Relay send
  → Kafka at-least-once
  → handler success then mark offset
  → Redis/DB/ES idempotent side effect
```

- Kafka 发送失败：Outbox pending 保留并重试。
- Kafka ack 后 Outbox mark 失败：可能重复发送，消费者必须幂等。
- Consumer 失败：不 mark offset，新 session 可以重投。
- Redis/ES 是加速或派生层；MySQL 真相不随其故障改变。

## 3. 新增证据代码

| File | Evidence |
|---|---|
| `internal/events/outbox_relay_test.go` | backlog recovery 与 Kafka-ack/DB-mark 重投窗口 |
| `internal/worker/consumer_group_test.go` | handler 失败不 mark，session restart 重投 |
| `internal/repository/note_cache_benchmark_e2e_test.go` | 真实 MySQL/Redis/local Note 路径 |
| `internal/service/feed_benchmark_test.go` | Pull/Push/Hybrid 算法与 allocation 对照 |
| `reliability_benchmark_e2e_test.go` | 隔离 Outbox→Kafka 和 ES/fallback 真实路径 |

## 4. Benchmark conclusions

- Note cache：真实本机数据中 Redis/local hit 明显避开 Docker-backed MySQL round trip；miss 仍承担 MySQL 成本。
- Feed：受控 harness 中 Push publish 随 follower 数增长，Pull publish 近似常数；这解释 Hybrid 的方向，但不能证明 1000 是最优阈值。
- Outbox/Kafka：隔离链路约 141.63ms/op；它包含顺序 DB 与 Kafka round trips，不代表吞吐上限。
- Search：一条数据的 warm ES 约 6.68ms/op，bounded MySQL fallback 约 43.83ms/op；小 fixture 只证明路径和边界。
- 完整 raw output、环境和命令见 M11 benchmark report。

## 5. Failure conclusions

1. Outbox 保证业务与待发事件原子，不保证 exactly-once。
2. 多 Relay claim/lease 仍是明确技术债；第一次共享 benchmark 已实际观察到竞争。
3. Consumer restart 能恢复未提交消息，但没有运行时 retry topic/DLQ。
4. Feed partial fanout 依赖 Redis ZSet 幂等与 handler 重投补齐。
5. ES down 有严格 MySQL fallback 边界；Redis down 会增加 MySQL 压力。

## 6. Documentation map

- Runtime architecture：`14_COMMUNITY_V1_ARCHITECTURE.md`。
- Event ownership/idempotency：`15_EVENT_CATALOG.md`。
- DB/Redis/ES：`16_DATA_CATALOG.md`。
- Failure/recovery：`17_FAILURE_MATRIX.md`。
- Entry index：`docs/community-dev/README.md`。

## 7. Trade-offs

- 没有为了“可靠性”接入不具备真实延迟语义的 Retry 原型；诚实记录边界优于伪完成。
- Benchmark 数据隔离提升可重复性，但本机顺序小样本不能替代容量测试。
- M11 只补测试和文档证据，没有修改生产业务链路，降低收尾阶段回归风险。
- Compose 是本地开发拓扑，不是生产 HA 部署模板。

## 8. Architecture Review

- [x] 无新业务、基础设施或运行时依赖。
- [x] benchmark/failure harness 位于对应层或根 E2E 层。
- [x] 随机 Kafka topic、MySQL schema、ES index 在 cleanup 中回收。
- [x] 未引入 utils/common/helpers、全局 mutable dependency 或反向依赖。
- [x] 文档明确区分真相源、派生数据、已接线与仅存在的原型。

## 9. Verification

- `go build ./...`：PASS。
- `go test ./...`：PASS。
- `go vet ./...`：PASS。
- `go test -tags=e2e ./...`：PASS。
- `git diff --check`：PASS。
- `go test -race ./...`：SKIPPED（本地 Windows 无受支持 C 编译器；用户明确禁止继续修改工具链）。

## 10. 我自己编码的三个任务

1. 给 Outbox Relay 设计基于 MySQL claim/lease 的多实例方案，先写状态机与故障窗口，不实现代码。
2. 用真实 Redis/MySQL 数据重跑 100/1k/10k follower Feed benchmark，并解释与受控 harness 的差异。
3. 为 notification-service 做一次手工进程中断/恢复实验，记录 consumer lag 与恢复时间，不新增监控组件。

> Agent 不提供本节答案，除非我完成尝试后请求 hint。
