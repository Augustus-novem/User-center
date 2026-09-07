# 13 目录蓝图与模块演进规则

## 目标

最终仓库应该能让第一次阅读的人在几分钟内知道：

- 启动入口在哪里；
- HTTP 在哪里；
- 业务编排在哪里；
- DB 在哪里；
- Kafka/Redis/ES 在哪里；
- IoC 在哪里；
- 每个业务模块增加了哪些文件。

## 推荐蓝图

> 这是目标职责图，不要求 M00 立刻重构成完全相同的目录。优先保持现有结构稳定。

```text
.
├── cmd/
│   ├── user-center/
│   ├── worker/
│   └── notification-service/
│
├── internal/
│   ├── config/
│   ├── domain/
│   ├── web/
│   │   ├── middleware/
│   │   ├── follow.go
│   │   ├── note.go
│   │   ├── feed.go
│   │   ├── ranking.go
│   │   └── search.go
│   │
│   ├── service/
│   │   ├── follow.go
│   │   ├── note.go
│   │   ├── feed.go
│   │   └── ...
│   │
│   ├── repository/
│   │   ├── follow.go
│   │   ├── note.go
│   │   └── ...
│   │
│   ├── dao/
│   ├── integration/
│   │   └── search/
│   └── events/
│
├── ioc/
├── script/
├── docs/
└── docker-compose.yaml
```

## 新功能落位示例：Follow

优先：

```text
internal/web/follow.go
internal/service/follow.go
internal/repository/follow.go
internal/dao/follow.go          # 仅现有项目需要时
```

不要：

```text
internal/follow/handlers/v1/
internal/follow/application/
internal/follow/infrastructure/
internal/follow/common/utils/
```

当前项目没有复杂到需要这种层级。

## 新功能落位示例：Elasticsearch

ES 是 infrastructure：

```text
internal/integration/search/
```

业务层只能依赖小接口：

```go
type NoteSearcher interface {
    SearchNotes(ctx context.Context, query SearchQuery) (...)
}
```

不能让 service 到处 import Elasticsearch SDK。

## Worker

Kafka handler 不要全部堆进一个 `worker.go`。

按事件职责拆文件，但共享统一消费框架：

```text
worker/
  note_published.go
  note_liked.go
  comment_created.go
```

公共幂等/重试/日志能力通过 wrapper 组合。

## 禁止目录模式

```text
internal/utils
internal/common
internal/shared
internal/helper
internal/base
internal/manager
```

如果 Agent 想创建这些目录，必须停止并请求用户批准。
