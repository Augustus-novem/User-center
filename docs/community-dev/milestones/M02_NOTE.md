# M02 多图笔记 + Outbox

## 目标
建立内容核心，并把现有 Outbox 扩展为通用社区事件基础设施。

## 分支
`feat/community-m02-note`

## 不做

- 不实现对象存储、图片上传、CDN。
- 不实现草稿箱、编辑接口、可见性（仅 published/deleted）。
- 不消费 `note.published`；本阶段只保证事件进入同一事务的 Outbox，由现有 Relay 投递。
- 不加 Redis 笔记缓存（那是 M07）。
- 不引入新目录或新依赖。

## 数据

```text
notes
- id
- author_id
- title
- content
- status          # published | deleted
- created_at
- updated_at

note_images
- id
- note_id
- url
- sort_order      # 从 0 开始，发布时按请求数组顺序写入
```

```text
INDEX(author_id, status, created_at, id)   # 作者主页列表 + 稳定游标
INDEX(note_id, sort_order)                 # 详情读图
```

第一版只保存图片 URL。

## API

全部需要登录。

```text
POST   /notes
GET    /notes/:id
DELETE /notes/:id          # 仅作者，软删除
GET    /users/:id/notes    # 只返回 published，cursor 分页
```

`POST /notes` 请求：

```text
{"title":"...","content":"...","image_urls":["https://..."]}
```

约束：title 必填且 ≤ 80；content ≤ 5000；图片 0~9 张，URL 非空。

## 发布事务

Kafka 开启时：

```text
BEGIN
  INSERT notes
  INSERT note_images
  INSERT event_outbox(topic=note.published)
COMMIT
```

必须走现有 `repository.Transaction.InTx` + `dbFromCtx`，与注册/签到同一套 Outbox。

Kafka 关闭时只写 notes/images，不写 Outbox（与现有 Signup 行为一致）。

删除不发事件。非作者删除返回业务错误，不改行。

## Event

```json
{"event_id":"...","type":"note.published","note_id":123,"author_id":10,"occurred_at":0}
```

Topic：`note.published`。Relay 已按 Outbox.topic 投递；worker 本阶段不订阅。

## 必测
发布、图片排序、事务失败不部分成功、删除/非作者删除、Outbox row 正确。
