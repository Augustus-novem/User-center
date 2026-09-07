# ADR-007：IoC 与横切能力

**Status:** Accepted

## IoC

继续使用 Wire 编译期依赖注入。

禁止 service locator、全局 mutable singleton 和 `init()` 建连接。

## Cross-cutting

不引入 Java AOP 框架。

- HTTP：Gin middleware。
- Service/Repository：Decorator。
- Kafka consumer：Handler wrapper / middleware chain。

横切能力只处理 logging、auth、rate limit、idempotency、retry、metrics 等通用问题，不承载具体业务判断。
