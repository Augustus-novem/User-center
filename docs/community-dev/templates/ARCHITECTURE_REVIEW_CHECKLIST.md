# Architecture Review Checklist

每个 milestone PR 都要检查。

## Dependency
- [ ] web 只向 service/必要 contract 依赖。
- [ ] service 不依赖 Gin/GORM/Viper/Wire。
- [ ] repository 不依赖 web/service。
- [ ] domain 不依赖基础设施 SDK。
- [ ] 无 import cycle。
- [ ] 新 package 的职责可一句话描述。

## Interface
- [ ] interface 有真实边界/测试价值。
- [ ] interface 方法最小化。
- [ ] 没有机械 `IXXX`。
- [ ] 没有巨型 repository/service interface。

## IoC
- [ ] dependency 由 constructor 注入。
- [ ] Wire/ioc 只装配，不承载业务。
- [ ] 无 service locator。
- [ ] 无 package global mutable dependency。

## Cross-cutting
- [ ] auth/log/recovery/rate-limit 在 middleware。
- [ ] idempotency/retry/logging 可用 wrapper 时没有散落复制。
- [ ] middleware/decorator 不含具体业务规则。

## Data
- [ ] transaction boundary 在 use-case 层可见。
- [ ] HTTP DTO 没有直接作为 persistence model。
- [ ] Redis/ES 是派生数据时有重建路径。

## Maintainability
- [ ] 没新增 utils/common/helpers 垃圾包。
- [ ] 没为了一个小模块创建过深目录树。
- [ ] 没发生无关大重构。
