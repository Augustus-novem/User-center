# ADR-005：Elasticsearch 故障采用有界降级

**Status:** Accepted

ES 故障后不能把所有搜索无条件转成 MySQL `%keyword%` 全库查询。

Fallback 必须限制时间范围/字段/limit，并设置 DB timeout；压力过大时允许直接返回搜索暂不可用，避免非核心搜索扩大故障半径。
