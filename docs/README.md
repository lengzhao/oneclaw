# oneclaw 文档

`oneclaw` 的文档主线不是“有哪些包”，而是“一个默认多 Agent、可持续学习和成长的运行时如何工作”。

维护者应先建立四个判断：

1. `oneclaw` 默认就是混合型多 Agent，而不是“先单 Agent，再按需升级”。
2. 不同角色可以使用不同模型，以同时获得高智商、低时延和低成本。
3. 执行与评价必须分离，默认避免 agent 对自己的结果做最终评价。
4. 系统成长依赖外部化记忆、知识、SOP、Skills，而不是模型权重训练。

## 推荐阅读顺序

1. [Agent Runtime 与学习闭环总览](./architecture/agent-runtime-and-learning-loop.md)
2. [默认自进化能力](./concepts/default-evolution.md)
3. [Memory 架构](./architecture/memory-architecture.md)
4. [Agent Profile 与任务路由](./concepts/agent-profiles-and-routing.md)
5. [运行时与会话模型](./architecture/runtime-and-session-model.md)
6. [ADR-003：任务编排信封与类型化元数据](./architecture/adr-003-orchestration-envelope.md)
7. [ADR-004：上下文装配流水线](./architecture/adr-004-context-assembly-pipeline.md)
8. [ADR-005：工作负载分级与队列优先级](./architecture/adr-005-workload-classes-and-priority.md)
9. [ADR-002：写回治理与 `WriteIntent` 流水线](./architecture/adr-002-write-governance.md)
10. [范围与非目标](./concepts/scope-and-non-goals.md)
11. [工作区布局](./reference/workspace-layout.md)
12. [配置参考](./reference/config-reference.md)
13. [快速开始](./reference/quickstart.md)
14. [运维速查与排错](./reference/runbook-troubleshooting.md)

## 文档分层

| 目录 | 作用 |
|------|------|
| `concepts/` | 系统目标、默认能力模型、角色分工和路由原则 |
| `architecture/` | 运行时结构、编排边界、上下文装配、治理约束 |
| `reference/` | 配置、运维、排障、使用方式 |
| `notes/` | 外部对照、路线图、调研和草稿性质材料 |

## 从哪里开始

- 想先理解整体目标：看 [Agent Runtime 与学习闭环总览](./architecture/agent-runtime-and-learning-loop.md)
- 想理解系统如何学习和沉淀：看 [默认自进化能力](./concepts/default-evolution.md)
- 想重点理解 memory 如何分层、召回和维护：看 [Memory 架构](./architecture/memory-architecture.md)
- 想理解默认多 Agent 如何分工：看 [Agent Profile 与任务路由](./concepts/agent-profiles-and-routing.md)
- 想理解请求如何进入运行时：看 [运行时与会话模型](./architecture/runtime-and-session-model.md)

## 默认能力模型

`oneclaw` 默认围绕两条主线工作：

1. **多 Agent 运行时**  
   由 `orchestrator`、若干执行型 worker 和独立 `reviewer` 组成混合型多 Agent 体系。
2. **外部化学习系统**  
   把经验沉淀到可审计的载体中，并在后续任务中按需召回。

这两条主线通过四类可进化载体闭环：

- **记忆**：对话轮次、摘要、用户偏好、任务结论
- **知识**：工作区文档、项目事实、设计说明
- **SOP**：流程约束、检查清单、排障步骤
- **Skills**：能力说明、风格约定、工具使用方法

## 文档状态约定

- `concepts/` 可以描述推荐默认形态，允许同时包含“当前方向”和“演进目标”。
- `architecture/` 中的 ADR 记录边界与约束，不直接承诺排期。
- `notes/` 仅作参考，不应被误读为已承诺交付。

新增 ADR 或总览文档时，应同步更新本页入口。

## 当前重点文档

- [Agent Runtime 与学习闭环总览](./architecture/agent-runtime-and-learning-loop.md)
- [Memory 架构](./architecture/memory-architecture.md)
- [运行时与会话模型](./architecture/runtime-and-session-model.md)
- [Agent Profile 与任务路由](./concepts/agent-profiles-and-routing.md)
- [ADR-002：写回治理与 `WriteIntent` 流水线](./architecture/adr-002-write-governance.md)
- [ADR-003：任务编排信封与类型化元数据](./architecture/adr-003-orchestration-envelope.md)
- [ADR-004：上下文装配流水线](./architecture/adr-004-context-assembly-pipeline.md)
- [ADR-005：工作负载分级与队列优先级](./architecture/adr-005-workload-classes-and-priority.md)
