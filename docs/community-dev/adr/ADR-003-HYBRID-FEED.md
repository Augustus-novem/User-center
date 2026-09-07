# ADR-003：Feed 使用 Pull → Push → Hybrid 演进

**Status:** Accepted

Push 阅读快但有大 V 写放大；Pull 发布轻但读取成本高。因此先 Pull baseline，再 Push，再 Hybrid。

没有 baseline 就无法量化 Hybrid 的 trade-off。`feed.fanout_threshold` 是配置项；默认 1000 只是本地起点，正式取值必须用后续 benchmark 解释，不能只引用“行业通常一万粉”。
