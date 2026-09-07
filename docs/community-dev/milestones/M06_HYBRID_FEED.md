# M06 Hybrid Feed

## 目标
针对大 V 写放大实现推拉结合。

## 分支
`feat/community-m06-hybrid-feed`

## 策略
配置 `feed.fanout_threshold`，不在业务函数硬编码。

```text
followers < threshold → fanout
followers >= threshold → 不 fanout
```

读取：

```text
push inbox candidates + followed celebrity recent notes → merge → stable sort → cursor
```

## 关键问题
celebrity 判定、threshold 更新、merge 去重、stable cursor、新关注大 V 的历史内容、取消关注后的旧 inbox 数据过滤。

## Benchmark
测试多个 follower 规模。threshold 用实验解释，不用“行业通常 1 万”作为唯一理由。

## 必测
普通作者、大 V、混合关注、duplicate note、取消关注、分页中途新内容。
