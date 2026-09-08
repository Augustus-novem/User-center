# M11 Reliability / Benchmark / 项目收尾

## 目标
停止扩业务，把已有设计变成有证据的简历项目。

## 分支
`chore/community-m11-reliability`

## Reliability
Outbox backlog 恢复、duplicate Kafka event、consumer restart、Redis/Kafka/ES down、cache stampede、Feed worker failure。

## Benchmark
至少完成：
1. Note detail cache 对照。
2. Pull/Push/Hybrid Feed 对照。
3. Outbox/Kafka 异步链路延迟。
4. Search ES/fallback 基本延迟。

## Documentation
architecture diagram、event catalog、Redis key catalog、DB schema、failure matrix、benchmark report、final README、learning index。

## 最终 tag

```bash
git tag -a community-v1 -m "community backend v1"
git push origin community-v1
```

community-v1 后禁止无目的堆技术；AI/Agent 是未来独立项目，不属于 `community-v1`。

## 实现结果

### Reliability evidence

- Outbox Kafka send 失败后保留 pending，下一次 dispatch 恢复发送。
- Kafka ack 后 MySQL mark 失败会重投，明确证明 at-least-once 重复窗口。
- Consumer handler 失败不 mark message；新 session 重投同一 offset 后成功。
- Redis down、cache stampede、Feed partial failure、ES down/timeout 已由既有和 M11 测试覆盖。
- 第一次共享 Outbox benchmark 真实触发多 Relay 标记竞争；M11 不扩大范围修复，改用隔离 schema 测量并把限制写入 failure matrix。

### Benchmark evidence

四组要求均完成并保存环境、fixture、命令、原始输出与解释：

1. Note detail：MySQL / Redis miss / Redis hit / local hit，并引用 M07 singleflight ON/OFF。
2. Pull / Push / Hybrid Feed：发布 follower scale 与 Top20 read 受控对照。
3. Outbox → Kafka：隔离 schema 与随机 topic 的真实 20 次链路。
4. Search：随机 ES index 与隔离 MySQL schema 的 warm ES / bounded fallback 真实 20 次对照。

记录：`docs/community-dev/benchmarks/2026-09-08-m11-community-v1.md`。

### Final documentation

- `14_COMMUNITY_V1_ARCHITECTURE.md`
- `15_EVENT_CATALOG.md`
- `16_DATA_CATALOG.md`
- `17_FAILURE_MATRIX.md`
- `README.md`
- `docs/community-dev/README.md`
- `M11_LEARNING_HANDOFF.md`

### Scope decision

M11 没有接入现有 `RetryableHandler`：其 Kafka timestamp 不构成 broker delayed delivery，直接接线会制造“已有 1/5/30 秒延迟重试”的错误结论。`.dlq` topic 仍仅是预创建名称，运行链路不宣称 Retry/DLQ。

## Verification（2026-09-08）

- Reliability unit tests：PASS。
- 四组 benchmark：PASS，原始结果已保存。
- Final build/test/vet/E2E/diff check：见 `M11_LEARNING_HANDOFF.md`。
- race：SKIPPED，本地 Windows 无受支持 C 编译器；按用户约束不修改系统环境。
