# M04 Pull-based Following Feed

## 目标
建立正确、简单、可对照的 Feed baseline。

## 分支
`feat/community-m04-pull-feed`

## 不做

- 不使用 Kafka fanout。
- 不使用 Redis Feed Inbox。
- 不实现 Hybrid / 大 V 阈值。
- 不编造 EXPLAIN / P95 数字。本阶段 baseline 是正确的 SQL 与稳定 cursor，性能对照留给 M11。

## API

```text
GET /feed/following?cursor=&limit=
```

需要登录。返回当前用户关注的人发布的 `published` 笔记。

## 逻辑

```text
notes n
JOIN user_relations r
  ON r.followee_id = n.author_id
 AND r.follower_id = current_user
WHERE n.status = 'published'
  AND (n.created_at, n.id) < cursor
ORDER BY n.created_at DESC, n.id DESC
```

Cursor：`created_at + "_" + id`。两页之间新插入的更新笔记会出现在下一轮首页，不会插入已翻过的旧页造成重复。

无关注：空列表。deleted 笔记不出现。

## 必测
无关注、多作者、相同 created_at、两页间插入新 Note、不重复不遗漏、deleted Note 不出现。
