# M02 多图笔记 + Outbox

## 目标
建立内容核心，并把现有 Outbox 扩展为通用社区事件基础设施。

## 分支
`feat/community-m02-note`

## 数据

```text
notes
- id
- author_id
- title
- content
- status
- created_at
- updated_at

note_images
- id
- note_id
- url
- sort_order
```

第一版只保存图片 URL，不实现对象存储。

## API
`POST /notes`、`GET /notes/:id`、`DELETE /notes/:id`、`GET /users/:id/notes`。

## 发布事务

```text
BEGIN
  INSERT note
  INSERT note_images
  INSERT outbox_event(note.published)
COMMIT
```

必须同一个 MySQL 本地事务。

## Event

```json
{"event_id":"...","type":"note.published","note_id":123,"author_id":10,"occurred_at":0}
```

此阶段消费者可暂时无业务动作；重点是事件可靠进入 Outbox/Relay。

## 必测
发布、图片排序、事务失败不部分成功、删除/非作者删除、Outbox row 正确。
