# 05 测试策略

测试目标是验证设计，不是堆覆盖率。

## Unit

Cursor encode/decode、Feed merge、Hot score、Cache key、Event validation、Service 纯逻辑。

## Repository Integration

真实 MySQL/Redis：唯一索引、SQL 排序分页、Redis ZSET、TTL、幂等 key。

## Event Integration

真实 Kafka：Outbox→Relay→Kafka、Consumer Group、重复 event、consumer 失败恢复。

## API Integration

HTTP → Handler → Service → Repository，验证 HTTP/business code、DB 状态和最终一致的派生状态。

## E2E

只覆盖少量关键链路：

```text
发布 Note → MySQL → Outbox → Kafka → Feed Inbox / Search Index / Notification
```

## 最终一致性

禁止固定 `time.Sleep(5s)`。实现测试 helper：`eventually(timeout, interval, condition)`。

## 幂等

至少测试 Follow/Like 重复、同 `event_id` 重复消费、Relay 重复投递、Feed fanout 重复、Search index 重复。

## 动态列表稳定性

Feed/Hot：相同时间戳、两页间插入新数据、cursor 边界、不重复、不遗漏。
