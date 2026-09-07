# M01 Follow 用户关注关系

## 目标
为 Feed 提供真实社交关系基础。

## 分支
`feat/community-m01-follow`

## 数据模型

```text
user_relations
- id
- follower_id
- followee_id
- created_at
```

约束：

```text
UNIQUE(follower_id, followee_id)
INDEX(follower_id, created_at, id)
INDEX(followee_id, created_at, id)
```

## API

```text
POST   /users/:id/follow
DELETE /users/:id/follow
GET    /users/:id/followers
GET    /users/:id/following
```

## 行为
- 不允许关注自己。
- 重复关注幂等；数据库不能有重复行。
- 重复取消定义稳定行为。
- cursor pagination。
- 此阶段不加 Redis。

## 必测
正常、自关注、重复/并发重复关注、取消/重复取消、followers/following stable cursor。

## 学习重点
unique index、check-then-insert race、cursor vs offset。
