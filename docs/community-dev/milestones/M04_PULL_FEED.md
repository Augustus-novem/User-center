# M04 Pull-based Following Feed

## 目标
建立正确、简单、可对照的 Feed baseline。

## 分支
`feat/community-m04-pull-feed`

## API
`GET /feed/following?cursor=...`

## 逻辑

```text
current user
→ following ids
→ published notes by authors
→ ORDER BY created_at DESC, id DESC
→ cursor
```

Cursor 至少包含 `created_at + id`，保证同时间戳稳定排序。

## 禁止
不使用 Kafka fanout；不使用 Redis Feed Inbox；不直接实现 Hybrid。

## 必测
无关注、多作者、相同 created_at、两页间插入新 Note、不重复不遗漏、deleted Note 不出现。

## Baseline
保存数据规模、关注数、SQL、EXPLAIN、P95（若压测），供 M05/M06 对照。
