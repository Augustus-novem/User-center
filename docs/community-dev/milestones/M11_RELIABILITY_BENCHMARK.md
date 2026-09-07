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
