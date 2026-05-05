# `workflows/*.yaml` 配置规格（v2）

本文是 oneclaw 当前唯一真源：`workflow_spec_version: 2`。  
执行内核基于 Eino `compose.Workflow`，用户层默认以字符串在节点间流转。

## 1. 顶层字段

必选：

- `workflow_spec_version: 2`
- `id`
- `nodes`（`map[string]Node`）

可选：

- `description`
- `defaults`（浅合并到每个节点 `params`）
- `meta`
- `steps`（线性语法糖；展开为 `nodes` + `depends_on`）
- `end`（最终收口节点；未填时默认最后一个非 async 节点）

## 2. Node 字段

`nodes.<id>` 支持：

- `use`：节点类型
- `agent_type`：`llm` / `agent_task` 可选或必需（见下）
- `input`：字符串模板输入
- `prompt`：字符串模板输入（优先于 `input`）
- `depends_on`：控制依赖
- `async`：后台 fire-and-forget（不阻塞主流程）
- `params`：扩展参数

## 3. 变量与默认数据流

- `$start.user_prompt`：本轮用户输入
- `$nodes.<id>`：等价 `$nodes.<id>.text`
- `$nodes.<id>.<field>`：读取上游结构化字段
- `$runtime.run_journal.current_turn_metadata`：当前轮运行元数据（用于 post-turn）

模板中的 `$nodes.*` 会自动推导依赖关系，不写 `depends_on` 也可形成正确先后关系。

## 4. 内置节点（v2）

- `on_receive`：入口校验/归一化
- `llm`：主 LLM/ADK 调用；不写 `agent_type` 时使用当前会话 agent
- `on_respond`：输出 assistant，并写入 transcript
- `agent_task`：显式子 agent 任务（必须有 `agent_type`）
- `retrieve_context` / `command` / `tool_call`：文本节点（默认返回可读文本）
- `noop`

## 5. 推荐默认模板

```yaml
workflow_spec_version: 2
id: default.turn
nodes:
  receive:
    use: on_receive
    input: $start.user_prompt
  main:
    use: llm
    prompt: $start.user_prompt
  respond:
    use: on_respond
    input: $nodes.main
  memory_agent:
    use: agent_task
    async: true
    agent_type: memory_extractor
    input: $runtime.run_journal.current_turn_metadata
    depends_on: [respond]
  skill_agent:
    use: agent_task
    async: true
    agent_type: skill_generator
    input: $runtime.run_journal.current_turn_metadata
    depends_on: [respond]
end: respond
```

## 6. 校验规则

- 仅接受 `workflow_spec_version == 2`
- `id`、`nodes` 必须存在
- `depends_on` 与模板 `$nodes.<id>` 引用必须指向存在节点
- 拒绝循环依赖
- `agent_task` 必须提供 `agent_type`

## 7. 运行结果

`TurnWorkflowResult` 当前仅包含：

- `assistant`
- `runtime`

不再返回节点输出索引（`nodes`）。

## 8. `manifest.yaml` 与 workflow 文件解析（oneclaw）

与 [`paths.CatalogRoot`](../paths/paths.go) 一致：**CatalogRoot = UserDataRoot**（默认 `~/.oneclaw`，可用 `config.user_data_root` 或环境变量 `ONECLAW_USER_DATA_ROOT` 覆盖）。**不存在**隐藏的 `.agent/` 子目录承载 manifest；**`manifest.yaml` 与 `agents/`、`workflows/`、`skills/` 平铺在 CatalogRoot 下**。

**`manifest.yaml`（子集）**：

- `default_agent`：默认主会话 Agent 的 **Catalog id**（与 `agents/<id>.md` 的**文件名 stem** 一致）。缺省或空串时，加载逻辑按 **`default`** 处理（与内置 / 模板一致）。
- `workflows.default_turn`：当 **不存在** `workflows/<agent_type>.yaml|.yml` 时，回落使用的 workflow **文件 stem**（不含扩展名），默认 **`default.turn`**。
- 兼容旧键：顶层 **`default_turn`** 与 `workflows.default_turn` 等价，**后者优先**（见 [`catalog.Manifest.ResolvedDefaultTurn`](../catalog/manifest.go)）。

**选用哪个 YAML 文件**（[`workflow.ResolveWorkflowPath`](../workflow/resolve.go)）：

1. 若存在 **`CatalogRoot/workflows/<agent_type>.yaml`** 或 **`.yml`**，则使用该文件（`<agent_type>` 为当前回合 Catalog 条目的 id，即 md **文件名 stem**）。
2. 否则使用 **`CatalogRoot/workflows/<default_turn>.yaml|.yml`**，其中 `<default_turn>` 来自 manifest（见上），默认可解析为模板 **`default.turn.yaml`**。

**未实现**：Agent frontmatter 中的 **`workflow:` / `chain:`** 别名覆盖；若将来加入，需同步更新解析与本文。

## 9. 修订记录

- 2026-05-05：切换到 v2（nodes/depends_on/string-flow），移除 v1 `graph.edges` 主模型；新增 §8（`manifest.yaml` 平铺、`ResolveWorkflowPath`、与实现对齐），原修订记录顺延为 §9。
