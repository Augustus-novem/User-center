# M06 Hybrid Feed：推拉结合

## 目标
在 M05 Push Inbox 上加入 celebrity 阈值，避免大 V 写放大。读路径合并 Inbox 与大 V 近况 Pull。

## 分支
`feat/community-m06-hybrid-feed`

## 不做

- 不编造 QPS / P95，也不用“行业通常 1 万”作为阈值唯一理由。
- 不实现 Elasticsearch、热榜、通知。
- 不把 celebrity 集合做成 Redis 真相；follower 数以 MySQL 为准。
- 不一次 load 全量 following / followers。

## 配置

```text
feed.fanout_threshold   # 默认 1000；<=0 表示关闭 Hybrid，行为回到 M05 全量扇出
```

1000 是本地可调起点：小账号走 Push，中等以上走 Pull。正式阈值必须靠后续 M11/压测解释，本 milestone 只保证语义正确。

## 写

```text
note.published
  → CountFollowers(author)
  → count >= threshold 且 threshold>0：跳过扇出，仍 MarkDone
  → count < threshold：M05 cursor 分批 Inbox fanout
```

## 读

```text
GET /feed/following
  → Inbox candidates（M05）
  → page following，按批 COUNT 出 celebrity followee
  → 每个 celebrity ListPublishedBefore(exclusive_max_note_id)
  → 按 note_id DESC merge 去重
  → 丢掉 deleted/unpublished/unfollow stale inbox
cursor = 本页最后一条有效笔记的 note_id
```

新关注大 V：Pull 能看到其近期已发布笔记，不依赖历史 Inbox。

取消关注：Pull 不再包含该作者；Inbox 脏 candidate 仍按 M05 过滤。

同一 note 同时出现在 Inbox 与 Pull：merge 按 id 去重。

## 必测
普通作者扇出、大 V 不扇出、混合关注 merge、duplicate note、取消关注、分页中途新内容、threshold<=0 退回全量扇出。
