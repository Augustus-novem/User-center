# ADR-006：Go 接口策略

**Status:** Accepted

## Decision

接口按“使用方需要的能力”定义，而不是按 concrete struct 机械生成。

## Rules

- 小接口优先。
- consumer-side interface 优先。
- infrastructure boundary 可接口化。
- 不创建 Java 风格 `IFooService`。
- 只有一个实现且没有测试/替换需求时，可以直接依赖 concrete type。
- constructor 默认返回 concrete type。

## Reason

过度接口化会让个人项目产生大量无意义文件，降低可读性，而不是提升架构质量。
