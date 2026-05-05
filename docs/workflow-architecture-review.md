# Workflow 架构评审（v2）

本文记录 oneclaw 从 v1 Graph 迁移到 v2 Workflow 的最终状态，作为当前实现口径。

## 1. 架构结论

- 解析层：`workflow.ParseBytes` 只接受 `workflow_spec_version: 2`，主模型为 `nodes` + `depends_on`（`steps` 仅语法糖）。
- 执行层：`wfexec.Execute` 统一走 `CompileEinoWorkflow`，底座是 Eino `compose.Workflow`。
- 数据流：用户层默认字符串流转；`$nodes.<id>` 等价 `$nodes.<id>.text`。
- 运行时：`RuntimeContext` 作为节点执行环境注入，非 Workflow 主 I/O。
- 返回结果：`TurnWorkflowResult` 仅保留 `assistant` 与 `runtime`，不再汇总 `nodes`。
- 异步：`async: true` 保持 fire-and-forget 语义，主链路不被 post-turn 任务阻塞。

## 2. 节点语义（当前）

- `on_receive`：入口检查与输入归一化。
- `llm`：主 LLM/ADK 节点；可选 `agent_type` 指向其它 agent。
- `on_respond`：以显式输入文本作为 assistant 输出并落 transcript。
- `agent_task`：显式子 agent 任务（必须 `agent_type`）。
- `retrieve_context` / `command` / `tool_call`：当前默认文本 passthrough 语义（保留扩展点）。

## 3. 默认模板形态

- `default.turn`：`on_receive -> llm -> on_respond -> async agent_task(memory/skill)`，`end: respond`。
- `memory_extractor` / `skill_generator`：`on_receive -> llm -> on_respond`。

## 4. 风险与后续

- `RuntimeContext` 仍承担较多共享状态，后续可继续收敛到更清晰的 turn state 分层。
- `retrieve_context` / `command` / `tool_call` 的高级结构化输出尚可继续增强。
- 当前已保证文档、模板与执行实现一致；后续新增 node 能力时需同步更新 [workflows-spec.md](workflows-spec.md)。

## 5. 迁移流程图

```mermaid
flowchart LR
  A[v1 graph/edges] --> B[workflow v2 nodes/depends_on]
  B --> C[wfexec CompileEinoWorkflow]
  C --> D[Eino compose.Workflow]
  D --> E[TurnWorkflowResult]
```

## 6. 变更记录

- 2026-05-05：完成 v2 迁移，移除 v1 Graph 主执行路径与旧默认模板。
