# 01 目标架构

```text
                                  +------------------+
                                  |      Client      |
                                  +--------+---------+
                                           |
                                           v
                                  +------------------+
                                  |  user-center API |
                                  |   Gin / Service  |
                                  +--+----+----+----+
                                     |    |    |
                 +-------------------+    |    +-------------------+
                 |                        |                        |
                 v                        v                        v
             MySQL                    Redis                  Outbox Events
      User/Follow/Note/...      Cache/Feed/Ranking               |
                                                                  v
                                                               Relay
                                                                  |
                                                                  v
                                                                Kafka
                         +-------------------+---------------------+------------------+
                         |                   |                                        |
                         v                   v                                        v
                   Feed / Hot Worker   Search Index Worker                  Notification Worker
                         |                   |                                        |
                         v                   v                                        v
                       Redis           Elasticsearch                              MySQL/Redis
```

## 数据真相

- User / Follow / Note / Like / Comment：MySQL。
- Feed Inbox：Redis 派生数据，可重建。
- Hot Ranking：Redis 派生数据，可重建。
- Elasticsearch：派生索引，可重建。
- Notification：最终建议 MySQL 为主。
- Outbox：MySQL 中可靠事件待投递记录。

## 一致性

| 场景 | 模型 |
|---|---|
| 发布后查询笔记详情 | 强/读己之写 |
| 发布后粉丝 Feed 出现 | 最终一致 |
| 发布后 ES 可搜索 | 最终一致 |
| 热榜更新 | 最终一致 |
| 关注关系 | MySQL 强约束 |
| 多级缓存 | 允许明确的短 stale window |

## community-v1 不做

Kubernetes、注册发现、gRPC、分库分表、CDN/对象存储系统、大型推荐算法、分布式事务框架、多地域容灾。
