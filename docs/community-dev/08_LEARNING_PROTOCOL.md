# 08 系统完成后的学习协议

## 第一轮：Git 时间线

```bash
git diff community-base..community-m01
git diff community-m01..community-m02
...
```

每阶段回答：新增什么业务问题？新增什么数据结构？新增什么系统复杂度？为什么上一阶段不够？

## 第二轮：调用链

自己画：

```text
HTTP → Handler → Service → Repository → DB/Redis → Outbox → Kafka → Consumer
```

## 第三轮：自己改代码

每个 milestone 的 handoff 留 3 个小任务。先自己写，卡 20~30 分钟再让 Agent 给 hint，不直接要完整答案。

## 第四轮：故障推演

Redis/Kafka/ES 挂了？Consumer 重复？Relay 重复投递？先口述，再实际验证。

## 第五轮：面试复述

每模块 3 分钟：问题 → baseline → 瓶颈 → 优化 → trade-off → 验证数据。

## 最终应该能回答

- Follow 为什么需要 unique index？
- Feed 为什么 cursor 而不是 offset？
- Push Feed 写放大是什么？
- Hybrid 如何降低大 V 写放大？
- Outbox 为什么仍可能重复消息？
- singleflight 防什么？
- 空值缓存和 TTL jitter 分别解决什么？
- Hot ranking 为什么用时间桶？
- ES down 为什么不能无限 fallback MySQL？
