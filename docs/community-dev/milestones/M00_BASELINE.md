# M00 基线稳定与冻结（已完成）

## 目标

在新增社区业务前，得到可构建、可测试、可运行、无嵌入式智能服务依赖的 Go 后端基线。

## 已完成

- 本地 secret 停止跟踪并加固 ignore。
- Go 主服务解除外部智能服务依赖。
- Compose 只保留 MySQL、Redis、Kafka 和三个 Go 服务。
- `cmd/compensate-job` 恢复编译。
- 宿主机与容器 Kafka listener 地址统一。
- Signup、Login、Checkin、Outbox、Kafka consumer、Redis 排行榜真实链路通过。
- `go build ./...`、`go test ./...`、`go vet ./...`、race tests 通过。
- 架构治理文档安装到正式位置。

## 后续规则

M00 在 `community-base` tag 创建后永久结束，不新增 M00 审计阶段。普通技术债记录在 `00_CURRENT_BASELINE.md`，只能在对应 milestone 或独立获批任务中处理。
