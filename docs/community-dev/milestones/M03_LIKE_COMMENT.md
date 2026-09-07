# M03 Like / Comment

## 目标
完成社区交互基础。

## 分支
`feat/community-m03-engagement`

## 不做

- 不把 Like 做成 Redis 计数 + MQ 最终一致。
- 不实现评论回复树、点赞评论、审核流。
- 不消费 `note.liked` / `comment.created`；只写入 Outbox 供 M08/M10。
- Unlike 不发事件。

## 表

```text
note_likes
- id
- user_id
- note_id
- created_at
UNIQUE(user_id, note_id)

comments
- id
- note_id
- user_id
- content
- status          # published | deleted
- created_at
INDEX(note_id, status, created_at, id)
```

## API

```text
POST   /notes/:id/like
DELETE /notes/:id/like
POST   /notes/:id/comments
GET    /notes/:id/comments   # created_at ASC, id ASC cursor
```

重复点赞 / 并发点赞：唯一索引冲突视为已点赞，且不写第二封 `note.liked`。
重复取消：DELETE 0 行仍成功。
笔记不存在：不能点赞或评论。
评论：content 去空格后非空，长度 ≤ 500。

## Outbox

同一事务：

```text
like    → note.liked      {event_id,type,note_id,user_id,occurred_at}
comment → comment.created {event_id,type,comment_id,note_id,user_id,occurred_at}
```

## 必测
重复/并发 Like、Unlike、不存在 Note、Comment 空值/长度边界、cursor。
