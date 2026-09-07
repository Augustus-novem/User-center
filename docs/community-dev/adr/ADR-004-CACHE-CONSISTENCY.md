# ADR-004：多级缓存接受短时间最终一致

**Status:** Accepted

Note detail 可使用 local + Redis 二级缓存，MySQL 仍是真相。更新/删除主动 invalidation Redis，本地缓存使用短 TTL 或可控 invalidation。

系统允许明确的短 stale window，不声称强一致缓存。
