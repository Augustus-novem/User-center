# M03 Like / Comment

## 目标
完成社区交互基础。

## 分支
`feat/community-m03-engagement`

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
- status
- created_at
```

## API
`POST /notes/:id/like`、`DELETE /notes/:id/like`、`POST /notes/:id/comments`、`GET /notes/:id/comments`。

第一版 MySQL 正确性优先，不把 Like 一开始全部 Redis+MQ 化。

基础稳定后通过 Outbox 发 `note.liked`、`comment.created`，供 M08/M10 使用。

## 必测
重复/并发 Like、Unlike、不存在 Note、Comment 空值/长度边界、cursor。
