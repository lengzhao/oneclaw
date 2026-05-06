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
- `meta`：执行层可读扩展；常用键：
  - **`transcript_mode: summary`** —— `*_transcript.jsonl` 只写简短占位行（完整对话仍在 Run Journal）；默认不传或为其他值则 transcript 写全文。等价别名 **`transcript_summary: true`**。
  - **`user_prompt_prefix`** / **`user_prompt_suffix`**（字符串，可用 YAML `|` 多行）——在送入模型、Run Journal 的 `user_message`、结构化抽取回退对话之前，对「本轮原始用户输入」做首尾拼接；中间空一行。**检索类上下文（如 MemoryRecall）仍只对原始输入查**，`$start.user_prompt` 模板仍为原始输入，避免翻译口令等污染召回。
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
- 入站若带 `MediaPaths`（宿主映射到 runtime），`adk_main` 会先把可读本地附件复制到 **`workspace/inbound/`**，并在当前用户消息中附加 **Context attachment** 路径列表；若模型判定支持图像输入，还会以内联多模态图片 part 一并传入。
- `$nodes.<id>`：等价 `$nodes.<id>.text`
- `$nodes.<id>.<field>`：读取上游结构化字段
- `$runtime.run_journal.current_turn_metadata`：**兼容占位**：当前实现等价于 **`correlation_id` 字符串**（历史模板仍可用；新模板请改用 **`$runtime.post_turn.ctx`**）。
- `$runtime.post_turn.ctx`：宿主回合 **PostTurn YAML**（`post_turn_ctx` 信封，含 `run_journal.path`、`host_agent_id`、`session_root` 等），供 **`agent_task`** 传给 `memory_extractor` / `skill_generator`。

模板中的 `$nodes.*` 会自动推导依赖关系，不写 `depends_on` 也可形成正确先后关系。

## 4. 内置节点（v2）

- `on_receive`：入口校验/归一化
- `llm`：主 LLM/ADK 调用；不写 `agent_type` 时使用当前会话 agent
- `on_respond`：输出 assistant，并写入 transcript（**勿**用固定字符串 `input` 代替 `$nodes.<llm>`：`input` 会覆盖本轮 `assistant`，Run Journal 也会变成占位符；折叠 transcript 请用顶层 **`meta.transcript_mode: summary`**）
- `agent_task`：显式子 agent 任务（必须有 `agent_type`）
- `structured_memory_extract`：读取 PostTurn ctx / journal 路径对应 JSONL，调用 **`structuredmem.AppendExtractJournal`**（确定性，不经 LLM 调度）
- `command`：在 **当前会话 workspace**（`EffectiveWorkspacePath`）下执行 shell，语义与内置工具 **`exec`** 一致：**必须**已在 `config.yaml` → **`tools.exec`** 中显式 `enabled: true` 且命令命中 **`allow` 前缀**（并遵守 **`deny`**）；默认超时 30s，可通过 **`params.timeout_seconds`** 覆盖（上限 120）。命令取自 **`input`/`prompt` 模板**，若为空则尝试 **`params.command`** / **`params.shell`**。
- `tool_call`：对 **`RuntimeContext.ToolRegistry`** 中已注册的工具执行一次 **`InvokableRun`**。**`params.tool`**（或 **`params.name`**）为工具名；参数为合法 JSON 对象字符串，来自 **`input`/`prompt`**，若为空则尝试 **`params.arguments`** / **`params.args`**，再否则为 **`{}`**。**禁止**从 workflow 调用 **`run_agent`**（请使用 **`agent_task`**）。
- `retrieve_context`：文本透传（占位 / 文档化节点 ID），不执行 I/O。
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
    input: $runtime.post_turn.ctx
    depends_on: [respond]
  skill_agent:
    use: agent_task
    async: true
    agent_type: skill_generator
    input: $runtime.post_turn.ctx
    depends_on: [respond]
end: respond
```

**翻译 / 任务收口（节选）**：在顶层增加 `meta.user_prompt_suffix`（或 `user_prompt_prefix`）即可；`llm` 仍可写 **`prompt: $start.user_prompt`**（占位仍是用户原文，不与后缀重复拼接）。

```yaml
meta:
  user_prompt_suffix: |
    将上面的用户内容翻译成英文。只输出译文。
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

## 8. `config.yaml` 的 `catalog` 块与 workflow 文件解析（oneclaw）

与 [`paths.CatalogRoot`](../paths/paths.go) 一致：**CatalogRoot = UserDataRoot**（默认 `~/.oneclaw`，可用 `config.user_data_root` 或环境变量 `ONECLAW_USER_DATA_ROOT` 覆盖）。**`agents/`、`workflows/`、`skills/` 等声明式资产平铺在 CatalogRoot**；**默认 Agent 与默认 turn 回落 stem 写在 `UserDataRoot/config.yaml` 的 `catalog:` 下**（见 [`config.File`](../config/file.go)、[`config.CatalogConfig`](../config/catalog.go)）。

**`catalog:`（子集）**：

- `default_agent`：默认主会话 Agent 的 **Catalog id**（与 `agents/<id>.md` 的**文件名 stem** 一致）。缺省或空串时按 **`default`**（见 [`File.ResolvedDefaultAgent`](../config/catalog.go)）。
- `workflows.default_turn`：当 **不存在** `workflows/<agent_type>.yaml|.yml` 时，回落使用的 workflow **文件 stem**（不含扩展名），默认 **`default.turn`**（见 [`File.ResolvedDefaultTurn`](../config/catalog.go)）。

**选用哪个 workflow YAML**（[`workflow.ResolveWorkflowPath`](../workflow/resolve.go)）：

1. 若存在 **`CatalogRoot/workflows/<agent_type>.yaml`** 或 **`.yml`**，则使用该文件（`<agent_type>` 为当前 Catalog 条目的 id，即 md **文件名 stem**）。
2. 否则使用 **`CatalogRoot/workflows/<default_turn>.yaml|.yml`**，其中 `<default_turn>` 来自 **`config.catalog.workflows.default_turn`**，默认可解析为模板 **`default.turn.yaml`**。

**未实现**：Agent frontmatter 中的 **`workflow:` / `chain:`** 别名覆盖；若将来加入，需同步更新解析与本文。

## 9. 修订记录

- 2026-05-06：`command` / `tool_call` 节点 **真正实现**：shell（等同 **`exec` 策略**）与 **`ToolRegistry` 单次工具调用**；`retrieve_context` 仍为透传。
- 2026-05-06：§1 `meta` 补充 **`transcript_mode: summary`**（及 `transcript_summary` 别名）、**`user_prompt_prefix` / `user_prompt_suffix`**；§3 **`$runtime.post_turn.ctx`**（并澄清 **`$runtime.run_journal.current_turn_metadata`** 为兼容占位）；§4 **`structured_memory_extract`**、`on_respond` 说明；§5 默认模板 PostTurn 输入改用 **`$runtime.post_turn.ctx`**。
- 2026-05-05：切换到 v2（nodes/depends_on/string-flow），移除 v1 `graph.edges` 主模型；新增 §8（`ResolveWorkflowPath`、`config.catalog`、与实现对齐），原修订记录顺延为 §9；**独立 `manifest.yaml` 已删除**，默认 Agent / 默认 turn 回落并入 **`config.yaml` → `catalog:`**。
