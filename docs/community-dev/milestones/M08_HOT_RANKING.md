# M08 Redis 滑动窗口热榜

## 目标
把签到排行经验迁移到内容实时热榜。

## 分支
`feat/community-m08-hot-ranking`

## 输入事件
`note.published`、`note.liked`、`comment.created`，由 Kafka consumer 聚合。

## Redis
每分钟一个 ZSET bucket：

```text
hot:note:202609072010
hot:note:202609072011
```

member=note_id，score=interaction score。

## Query
最近 N 个 bucket → merge/aggregate → Top K。

## Stable Snapshot
第一页生成 `snapshot_minute`，后续页继续用同 snapshot，避免窗口滑动造成重复/遗漏。

## Score
配置化 publish/like/comment weight，先简单明确，不伪装推荐算法。

## 必测
bucket boundary、duplicate event、snapshot pagination、Kafka down、expired bucket。
