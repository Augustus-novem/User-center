# 09 Coding Agent Prompt 模板

## A. Milestone 开始：只读设计

```text
当前任务：<MXX 名称>。
请先阅读 AGENTS.md、对应 milestone 文档和相关代码。
现在禁止修改代码。

输出：
1. 当前相关调用链
2. 准备修改的文件
3. 新增文件
4. DB schema/index 变化
5. Redis key 变化
6. Kafka topic/event 变化
7. 一致性和并发风险
8. 测试计划
9. 不确定项

不得实现后续 milestone、不得大规模重构、不得引入未批准依赖。
```

## B. 批准后实现

```text
设计已批准。只实现当前 milestone 中的 <具体子任务>。
不修改无关模块；复用现有分层；保持可编译；补必要测试；完成后不要自动 commit。
输出修改文件、测试命令、尚未解决问题。
```

## C. Code Review

```text
现在不要写新功能。Review 当前 git diff，重点检查：业务正确性、并发、事务、幂等、context/timeout、索引/N+1、Redis key/TTL、Kafka MarkMessage、错误吞掉、false positive、过度设计。按 Critical/Major/Minor 分类。
```

## D. 学习交接

```text
不要修改代码。根据刚完成 milestone：
1. 画 HTTP 到存储/消息队列完整调用链
2. 列核心 struct/interface/function
3. 说明输入、输出、职责
4. DB 读写与索引
5. Redis key/结构/TTL
6. Kafka event/topic/group
7. 5 个故障场景
8. 10 个面试追问
9. 给我 3 个自己编码的小修改任务，不给答案
```

## E. Debug

```text
不要先改代码。根据现象列最多 3 个最可能根因；给验证每个假设的最小命令/观察点；等待验证结果后再修改。
```
