# M01 Follow 用户关注关系

## 目标
为 Feed 提供真实社交关系基础。

## 分支
`feat/community-m01-follow`

## 不做

- 不加 Redis 计数或关系缓存。
- 不发 Kafka / Outbox 事件（Feed 消费关注关系放到后续 milestone）。
- 不返回粉丝/关注数聚合字段。
- 不返回用户资料卡片，列表只返回 `user_id` + `created_at`。
- 不引入新中间件、新目录或新依赖。

## 数据模型

```text
user_relations
- id
- follower_id
- followee_id
- created_at   # Unix milli，与现有表时间字段一致
```

约束：

```text
UNIQUE(follower_id, followee_id)                 # 幂等关注的真相
INDEX(follower_id, created_at, id)               # following 稳定游标
INDEX(followee_id, created_at, id)               # followers 稳定游标
```

MySQL 是关注关系的唯一真相。重复关注的正确性由唯一索引保证，而不是应用层先查再插。

## API

全部需要登录。`:id` 是目标用户。

```text
POST   /users/:id/follow      # 当前用户关注 :id
DELETE /users/:id/follow      # 当前用户取消关注 :id
GET    /users/:id/followers   # :id 的粉丝
GET    /users/:id/following   # :id 的关注
```

列表查询参数：

```text
cursor  可选，上一页返回的 next_cursor
limit   可选，默认 20，最大 50
```

列表响应：

```text
{
  "items": [{"user_id": 2, "created_at": 1710000000000}],
  "next_cursor": "1710000000000_2",
  "has_more": true
}
```

## 行为

| 场景 | 行为 |
|---|---|
| 关注自己 | 业务错误：不能关注自己 |
| 关注不存在的用户 | 业务错误：用户不存在 |
| 首次关注 | 插入一行 |
| 重复关注 / 并发重复关注 | 成功且仍只有一行（唯一索引冲突视为已关注） |
| 取消已存在的关注 | 删除该行 |
| 重复取消 / 取消不存在的关系 | 成功，结果仍是未关注 |
| 列表 | `created_at DESC, id DESC` 稳定排序 + keyset cursor |

Cursor 编码为 `created_at + "_" + id`。相同毫秒下用 `id` 打破并列，避免 offset 分页在插入新行时跳过或重复。

## 调用链

```text
HTTP /users/:id/follow
  → web.FollowHandler
  → service.FollowService
  → repository.FollowRepository
  → dao.GORMFollowDAO / user_relations
```

关注时 service 额外通过已有 `UserRepository.FindByID` 确认 followee 存在。事务边界：单行 INSERT/DELETE，不需要跨表事务。

## 必测
正常、自关注、重复/并发重复关注、取消/重复取消、followers/following stable cursor。
