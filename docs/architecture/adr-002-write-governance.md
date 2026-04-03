# ADR-002：写回治理与 `WriteIntent` 流水线

## 状态

提议。

与本提议直接相关的上位文档：

- [默认自进化能力](../concepts/default-evolution.md)
- [范围与非目标](../concepts/scope-and-non-goals.md)
- [ADR-001：claw 模块边界与接口形态](./adr-001-module-boundaries.md)

## 背景

`oneclaw` 的核心方向之一，是把“自我进化”落在外部化载体上，例如记忆、知识、SOP、Skills 与工作区文件。

当前文档已经明确了这些原则：

- 低风险写回可尽量自动化
- 高风险写回必须可审计、可回滚
- 默认不要让 Agent 无约束地改写核心规则文件

但在已采纳边界里，写回仍然主要表现为“某个工具直接写文件”或“后台分析直接产出结果”。这会带来三个问题：

1. 治理逻辑分散在工具、宿主、后台任务中，难以统一。
2. 审计字段与审批策略容易退化成口头约定。
3. 后续多 Agent 增长后，不同角色的写回风险无法稳定收敛。

## 决策

将“写回”建模为独立的一等能力，并通过统一流水线执行：

```mermaid
flowchart LR
  I[WriteIntent] --> P[PolicyCheck]
  P --> A[Audit]
  A --> G[Optional Gate]
  G --> E[Apply]
  E --> R[Rollback Handle]
```

### 1. 所有高于“低风险追加”的写回，都应先生成 `WriteIntent`

`WriteIntent` 是逻辑写回意图，不等同于直接文件写入。

建议最少字段：

- `intent_id`
- `trace_id`
- `session_id`
- `actor_type`：如 `tool`、`background_agent`、`profile`
- `actor_name`
- `target_kind`：如 `workspace_file`、`skill_file`、`memory_entry`
- `target_path`
- `operation`：如 `append`、`replace`、`patch`
- `risk_level`
- `summary`
- `proposed_content` 或 `diff`
- `reason`

### 2. 低风险与高风险写回分流

默认建议：

| 类型 | 示例 | 默认策略 |
|------|------|----------|
| 低风险追加 | `memory/insights.md`、review critique、事实摘要 | 可自动执行，但仍需审计 |
| 中风险改写 | `USER.md`、部分 `skills/*.md` | 走策略检查，必要时轻审批 |
| 高风险改写 | `IDENTITY.md`、`AGENTS.md`、组织级 SOP | 默认禁止自动执行，需显式审批 |

### 3. 策略检查必须集中化

`PolicyCheck` 应至少检查：

- 路径是否在 allowlist 中
- 操作者是否有权限触达该路径
- 操作类型是否被允许
- 风险级别与目标文件是否匹配
- 是否需要评测、审批或人工确认

策略判断不应散落在每个工具实现里。

### 4. 审计是硬约束，不是可选项

每次写回都应记录：

- 发起方
- `trace_id`
- 原因
- 目标
- 执行结果
- 回滚句柄或版本信息

### 5. 回滚能力必须前置设计

版本管理与回滚能力默认直接依赖 `git`。

推荐约定：

- 工作区、Skills、SOP 等可进化载体默认纳入同一个 git 仓库
- 重要写回在执行前后都应具备可追踪 diff
- 高风险写回应能明确对应到一次 git 变更或一组相关变更
- 回滚默认通过 git 完成，而不是额外自造快照机制

“先写进去，之后再想怎么回滚”不符合默认自进化的设计目标。

## 对现有架构的影响

### 对 `tools`

工具不再直接承担全部治理职责。高风险写回型工具应优先返回 `WriteIntent`，由宿主或治理层继续处理。

### 对 `background_agent`

后台分析默认只能落低风险追加型写回；若需要改写中高风险文件，必须经过同一治理流水线。

### 对 `host`

宿主是最合适的治理落点。是否执行、是否审批、是否回滚，应由宿主集中决策，而不是塞回 `agent.Loop`。

## 后果

### 优点

- 把“默认安全”从文档原则变成架构能力。
- 多 Agent 增长后，治理逻辑不会失控分散。
- 更容易把审计、评测、审批和 git 回滚串成统一链路。

### 代价

- 写回路径从“直接写文件”变为“两阶段提交”式体验，链路更长。
- 宿主需要承担更多治理责任。

## 非目标

- 不要求首版就实现复杂审批 UI。
- 不要求所有低风险追加都走重型人工审批。
- 不把治理逻辑硬塞进 `agent.Loop` 本体。

## 推荐后续实现

1. 先在宿主侧引入 `WriteIntent` 数据结构。
2. 为 `memory/` 与工作区文件区分默认风险等级。
3. 把 `background_agent.output_file` 限定为低风险目录的默认示例。
4. 后续再补基于 git 的审批、评测与回滚实现。
