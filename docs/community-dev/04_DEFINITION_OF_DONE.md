# 04 Definition of Done

任何 milestone 必须全部满足才允许合并。

## 设计
- [ ] 需求边界明确。
- [ ] 明确“不做什么”。
- [ ] DB schema / index 已说明。
- [ ] Redis key 已说明（适用时）。
- [ ] Kafka event/topic 已说明（适用时）。
- [ ] 一致性和失败语义已说明。

## 架构
- [ ] 调用链遵循 `web → service → repository → dao/integration`，或已通过 ADR 说明例外。
- [ ] domain/service 未依赖 Gin、GORM、Viper、Wire 等不应泄漏的框架。
- [ ] IoC 仍集中在 Wire/ioc，无 Service Locator / 全局 mutable dependency。
- [ ] 新 interface 是最小、真实边界，不是机械抽象。
- [ ] 横切能力使用 middleware/decorator/wrapper，不复制到各业务函数。
- [ ] 无 `utils/common/helpers/misc/shared/base/core` 万能 package。
- [ ] 新目录职责和 import 方向清晰。
- [ ] HTTP DTO 与 persistence model 没有无边界混用。
- [ ] transaction boundary 可从 service/use-case 层看懂。
- [ ] `ARCHITECTURE_REVIEW_CHECKLIST.md` 已完成。

## 实现
- [ ] 无无关大规模重构。
- [ ] 无无理由新增依赖。
- [ ] context/timeout 合理。
- [ ] 关键日志结构化。
- [ ] 无 secret。

## 测试
- [ ] `go test ./...`
- [ ] `go vet ./...`
- [ ] `git diff --check`
- [ ] 并发模块 `go test -race ./...`
- [ ] happy path / invalid / duplicate / boundary。
- [ ] 关键失败路径。
- [ ] 涉及基础设施时 integration test。

## Git
- [ ] 小而可理解的 commit。
- [ ] PR 描述完整。
- [ ] 合并后 milestone tag。

## 学习
- [ ] Learning Handoff。
- [ ] 完整调用链。
- [ ] 核心 symbol。
- [ ] DB/Redis/Kafka 变化。
- [ ] 5 个故障问题。
- [ ] 10 个面试追问。
- [ ] 3 个用户自己写的小任务。

## 简历证据
- [ ] 指标真实。
- [ ] 保存 benchmark 环境、命令、原始输出。
