# M09 Elasticsearch 搜索 + 有界降级

## 目标
引入项目唯一一个新的重量级基础设施。

## 分支
`feat/community-m09-search`

## Index
`note_id`、`author_id`、`title`、`content`、`created_at`、`status`。

## Index Flow
禁止 HTTP 双写 MySQL+ES：

```text
note.published / note.updated / note.deleted → Kafka → search index worker → Elasticsearch
```

ES 是派生索引，可重建。

## API
`GET /search/notes?q=...`

## Reliability
必须有 request timeout 和 bounded fallback。DB fallback 限制时间范围/字段/limit，并有 DB timeout。不能让 ES down 导致全库 `%keyword%` 扫描拖垮主库。

## 必测
index eventually visible、update/delete、ES timeout/down、fallback boundary。
