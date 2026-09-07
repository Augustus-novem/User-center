# ADR-003：Feed 使用 Pull → Push → Hybrid 演进

**Status:** Accepted

Push 阅读快但有大 V 写放大；Pull 发布轻但读取成本高。因此先 Pull baseline，再 Push，再 Hybrid。

没有 baseline 就无法量化 Hybrid 的 trade-off。fanout threshold 是配置并通过 benchmark 解释。
