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
  - **`host_turn_nodes_yaml`**（字符串，多行 YAML）：**遗留**回落字段；仅当 `steps` 中无 `host_turn: true` 且无顶层 `host_turn_nodes` 时使用（见 §8）。
- **`host_turn_nodes`**（`map[string]Node`）：**遗留**回落字段；建议改用 `steps` + `host_turn: true`（见 §8）。
- `steps[*].host_turn`（bool，可选）：当为 `true` 时，该 step 不参与当前 workflow 的 `steps` 展开，而是仅在宿主加载 `default.turn` 时并入宿主图。
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
- **合并形态**：Eino Workflow 将上游 **`Text`/`Data`** 映射进下游时，除嵌套的 **`__node_text` / `__node_data`**（map：`__node_text[<id>]`、**`__node_data[<id>]`**）外，也会出现顶层键 **`__node_text.<id>`**、**`__node_data.<id>`**（与 **`compose.MapFields`** 带点路径一致）；模板解析对两种形态均支持。
- `$runtime.run_journal.current_turn_metadata`：**兼容占位**：当前实现等价于 **`correlation_id` 字符串**（历史模板仍可用；新模板请改用 **`$runtime.post_turn.ctx`**）。
- **`$runtime.run_journal.path`**：宿主本轮 **Run Journal** JSONL 绝对路径（`sessions/…/runs/<agent>/<correlation_id>.jsonl`）；会话根、`CorrelationID`、宿主 agent id 任一缺失则为空字符串。
- **`$runtime.user_data_root`**：当前 **`UserDataRoot`**（裁剪空白）。
- `$runtime.post_turn.ctx`：宿主回合 **PostTurn YAML**（`post_turn_ctx` 信封，含 `run_journal.path`、`host_agent_id`、`session_root` 等），供 **`agent_task`** 传给 `memory_extractor` / `skill_generator`。

模板中的 `$nodes.*` 会自动推导依赖关系（**含 `params` 下递归出现的字符串**），不写 `depends_on` 也可形成正确先后关系；跨 **`$nodes.<id>.<data 字段>`** 的数据依赖同理会从 **`prompt` / `input` / `params`** 中扫描出来。

## 4. 内置节点（v2）

- `on_receive`：入口校验/归一化
- `llm`：主 LLM/ADK 调用；不写 `agent_type` 时使用当前会话 agent
- `on_respond`：输出 assistant，并写入 transcript（**勿**用固定字符串 `input` 代替 `$nodes.<llm>`：`input` 会覆盖本轮 `assistant`，Run Journal 也会变成占位符；折叠 transcript 请用顶层 **`meta.transcript_mode: summary`**）
- `agent_task`：显式子 agent 任务（必须有 `agent_type`）。渲染后的 **`input`/`prompt`** 会作为子 Agent 的 **`$start.user_prompt`**。若渲染结果为空，则回落 **`BuildSubagentUserPrompt`**（拼用户句 + 主助手回复）。对 **`agent_type: memory_extractor`** / **`skill_generator`**：**若当前字符串无法解析出含非空 **`run_journal.path`** 的 PostTurn YAML**，实现会在宿主 **`RuntimeContext`** 上调用 **`BuildPostTurnCTXYAML`** 再试一次，以免异步调度或旧模板导致误用「Context for this agent run…」正文并拖垮 **`structured_memory_extract`** / Skills journal 门控。
- `if`：布尔门控节点；读取 **`params.when_any`**（非空数组），按 **OR** 求值，结果为 **`WorkflowNodeResult.Data["pass"]`**（bool）及 **`Text`** `true`/`false`。子句支持：**字符串**（整段当作模板渲染后再 truthy）；**`truthy:`** 单键映射；**`equals:`** `[left, right]`；**`gt:`** `[left, right]`（两侧渲染后按浮点数比较 **`>`**）。操作数可为字面量或 **`$…`** 模板片段。编译阶段会把 `if` 映射为 Eino 原生 `AddBranch`：默认将“依赖该 `if` 的后继节点”（**`depends_on`** 或 **`input`/`prompt`/`params`** 中的 **`$nodes.<if>`**）视为 **true 分支**，`pass=false` 时走 `END`。实现上：后继对 **`if`** 不再添加 **`AddDependency`**（否则会绕过分支）；若后继模板需要从 **`if`** 拉 **`Text`/`Data`**，编译器仅添加 **`WithNoDirectDependency`** 的数据映射（与 Eino Workflow 分支注释一致）。若图中仍存在 **`pass=false`** 时不可达的后继节点，建议在 Yaml 上为该后继补充 **`params.require_truthy`**（例如绑定 **`$nodes.<if>.pass`**），以避免与全局 **`START`** 入口边在 eager DAG 下的调度边角。
- `journal_tool_metrics`：汇总当前上下文可用的 Run Journal JSONL（见下）。默认路径：**会话根 + `CorrelationID` + 当前 workflow agent id**；当节点的 **`input`/`prompt`** 为 **`post_turn_ctx` YAML**（或单行的宿主 **`runs/<host>/<corr>.jsonl` 绝对路径**）时，改为读取 **`run_journal.path`**（宿主回合日志）。子 Agent（**`subs/`**）在无法从文本解析 PostTurn 时，优先使用 **`RuntimeContext.ParentSessionRoot`** + **`ParentAgentType`** + **`CorrelationID`** 定位宿主 **`runs/<parent>/<corr>.jsonl`**，避免误扫 **`runs/skill_generator/`** 等非宿主 journal 导致门控误判。可选 **`params.match_tools`**（字符串数组，精确匹配 **`tool_name`**）：结果中会多出 **`named_tools_present` / `named_tools_counts`**（全文 JSON），并在 **`Data`** 里附带 **`tool_matched_<工具名>`**（bool）与 **`tool_match_count_<工具名>`**（int）。结构化字段还包括：**`tool_call_events`**（`phase:"tool_call"` 行数，含重复 id）、**`line_count`**、**`duration_seconds`** / **`first_ts`** / **`last_ts`**（由行间 **`ts`** 推导跨度）、**`has_run_start` / `has_run_complete`**、**`tool_name_counts`**（按 **`tool_call_id`** 去重后的按工具计数）、原有 **`distinct_tool_calls`**、**`skill_tree_tool_used`**（命中 **`read_skill`**，或 **`read_file`/`write_file`** 路径命中 **`skills/`**）；**`Text`** 为完整 JSON。
- `structured_memory_extract`：读取 PostTurn ctx / journal 路径对应 JSONL，调用 **`structuredmem.AppendExtractJournal`**（确定性，不经 LLM 调度）。**推荐**将 **`input`/`prompt`** 设为 **`$start.user_prompt`**（与宿主 **`agent_task`** 传入的 PostTurn YAML 一致）；若使用 **`$nodes.receive`**，须确保上游 **`Text`** 已通过 Eino **`MapFields`** 合并进下游输入（参见 §3 **合并形态**）。当 **`$start.user_prompt`** 无法解析出 **`run_journal.path`**（例如 **`agent_task`** 回落为 **`BuildSubagentUserPrompt`**）且当前 **`RuntimeContext`** 来自 **`ForkSubAgentRuntime`**（带子会话 **`subs/`**）时，实现会用记录的 **`ParentSessionRoot`**、**`ParentAgentType`** 与 **`CorrelationID`** 回退解析宿主 **`runs/<parent>/<corr>.jsonl`**。
- `command`：在 **当前会话 workspace**（`EffectiveWorkspacePath`）下执行 shell，语义与内置工具 **`exec`** 一致：**必须**已在 `config.yaml` → **`tools.exec`** 中显式 `enabled: true` 且命令命中 **`allow` 前缀**（并遵守 **`deny`**）；默认超时 30s，可通过 **`params.timeout_seconds`** 覆盖（上限 120）。命令取自 **`input`/`prompt` 模板**，若为空则尝试 **`params.command`** / **`params.shell`**。若 **`params.parse_json_stdout: true`** 且标准输出为一层 JSON **对象**，则解析结果并入节点 **`Data`**（键可用于 **`$nodes.<id>.<field>`**）；**`Text`** 仍为原始 stdout。
- **`params.require_truthy`（通用节点门控）**：任意节点可选。值为模板字符串（可含 **`$nodes.*`**）；渲染后为 **truthy**（`true`/`1`/`yes`/`y`/`on`）才执行该节点，否则**跳过**（空输出，不报错）。常用于 `if` 后门控 `llm` / `on_respond` / `agent_task`。
- `tool_call`：对 **`RuntimeContext.ToolRegistry`** 中已注册的工具执行一次 **`InvokableRun`**。**`params.tool`**（或 **`params.name`**）为工具名；参数为合法 JSON 对象字符串，来自 **`input`/`prompt`**，若为空则尝试 **`params.arguments`** / **`params.args`**，再否则为 **`{}`**。**禁止**从 workflow 调用 **`run_agent`**（请使用 **`agent_task`**）。
- `retrieve_context`：文本透传（占位 / 文档化节点 ID），不执行 I/O。
- `noop`

## 5. 推荐默认模板

内置 **`default.turn`** 仅保留主干 **`receive → main → respond`** 与异步 **`memory_agent` / `skill_agent`**；宿主固定触发两个子 Agent。`skill_generator` 是否真正调用 `llm` 在其子 workflow 内部决定。

默认门控写在 **`skill_generator.turn`** 内部：使用 **`journal_tool_metrics`**（**`input: $start.user_prompt`** 绑定宿主 PostTurn 信封以读取宿主 journal），再通过 **`if`** 原生分支决定是否进入 `llm` 提取路径。

完整 YAML 见仓库 **`setup/templates/workflows/default.turn.yaml`** 与 **`setup/templates/workflows/skill_generator.yaml`**。

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

**宿主 `default.turn` 与 `skill_generator` 模板合并**：解析宿主 **`id: default.turn`** 后、**`Validate` 前**，运行时会读取 **`CatalogRoot/workflows/skill_generator.{yaml,yml}`**：优先从 `steps` 中抽取 **`host_turn: true`** 的 step 并按 step 语法糖展开；若无则回落顶层 **`host_turn_nodes`**；再无则回落 **`meta.host_turn_nodes_yaml`**（字符串）。并入宿主 **`nodes`**（同名 id 覆盖宿主中的定义）。子 Agent 加载 **`skill_generator.turn`** 时也会调用同一合并函数，但因 **`id ≠ default.turn`** 而为 **no-op**。

**`oneclaw init` 与嵌入式模板**：[`setup.Bootstrap`](../setup/bootstrap.go) 将嵌入式 **`setup/templates/**`** 拷入 **`UserDataRoot`** 时，对多数文件采用 **「缺失才写入」**（`copyTemplateIfMissing`）。已存在的 **`workflows/*.yaml`** **不会**随二进制升级自动覆盖；升级二进制后若要对齐 **`default.turn` / `memory_extractor` / `skill_generator`**，请执行 **`oneclaw init --upgrade-workflows`**（或 **`--user-data`** 指向自定义根），由 [`SyncWorkflowTemplatesFromEmbed`](../setup/bootstrap.go) **覆盖写入** `workflows/*.yaml`。亦可手动替换或删除对应 YAML 后再 **`oneclaw init`**。

**未实现**：Agent frontmatter 中的 **`workflow:` / `chain:`** 别名覆盖；若将来加入，需同步更新解析与本文。

## 9. 修订记录

- 2026-05-06：**`oneclaw init --upgrade-workflows`**：覆盖同步嵌入式 **`workflows/*.yaml`**，解决 **copy-if-missing** 导致旧目录长期沿用无 **`require_truthy`** 的 **`skill_generator`** 等问题；§8 说明。
- 2026-05-06：**`journal_tool_metrics`**：子 Agent 无 PostTurn 文本时优先用 **`ParentSessionRoot`** / **`ParentAgentType`** 解析宿主 journal；**`skill_generator.turn`** 的 **`main`（`llm`）** 增加 **`require_truthy: $nodes.skill_gen_if.pass`**，避免门控为假仍调用模型；§4 同步说明。
- 2026-05-06：**`structured_memory_extract`**：子 Agent runtime 携带 **`ParentSessionRoot`** / **`ParentAgentType`**（**`ForkSubAgentRuntime`**）时，在 PostTurn 文本缺失 **`run_journal.path`** 下回退定位宿主 journal；§4 同步说明。
- 2026-05-06：**`agent_task`**：对 **`memory_extractor`** / **`skill_generator`**，当渲染输入不含可解析的 **`run_journal.path`** 时，用宿主 **`BuildPostTurnCTXYAML`** 再试一次，以免异步枝误回落 **`BuildSubagentUserPrompt`** 拖垮 **`structured_memory_extract`** / Skills 门禁；§8 补充 **`oneclaw init`** **copy-if-missing** 与 workflows 手工升级说明。
- 2026-05-06：**`memory_extractor.turn`** 模板将 **`structured_memory_extract`** 的输入改为 **`$start.user_prompt`**，与子 Agent **`agent_task`** 的 PostTurn 载荷对齐，避免仅靠 **`$nodes.receive`** 时在部分图合并形态下得到空串。
- 2026-05-06：**`$nodes.<id>` / `$nodes.<id>.*` 模板解析**：除嵌套的 **`__node_data` / `__node_text`** map 外，兼容 Eino 将 **`MapFields`** 目标写成单个带点号的顶层键（**`__node_data.<id>`**、**`__node_text.<id>`**）时的合并结果；修复 **`if`** 中 **`gt`** 等对数值字段的门控在运行时误报 **`non-numeric operands`**。**`compose`** 侧 **`asGraphInputMap`** 兼容任意 **string-key** 的 **`reflect.Map`** 合并结果，避免输入图为 **`nil`** 导致 **`$nodes.*`** 解析落空。
- 2026-05-06：**`journal_tool_metrics`** 扩展（时长 / 行数 / `tool_call` 事件数 / `tool_name_counts` / `match_tools`）；子 Agent 可通过 **`input: $start.user_prompt`** 读取宿主 **`post_turn_ctx`** 指向的 journal。
- 2026-05-06：支持 `steps[*].host_turn: true`（`skill_generator` 声明宿主门控），并优先于 `host_turn_nodes` / `meta.host_turn_nodes_yaml`。
- 2026-05-06：**`host_turn_nodes`**（**`skill_generator`** 顶层 → merge 进宿主 **`default.turn`**）；**`meta.host_turn_nodes_yaml`** 保留为兼容回落。
- 2026-05-06：**`meta.host_turn_nodes_yaml`**（首批实现：**`skill_generator`** → merge 进宿主 **`default.turn`**）；**`$runtime.run_journal.path`** / **`$runtime.user_data_root`**；默认门控改为 **`command`(grep/wc) + `parse_json_stdout` + `if`**（见模板）。
- 2026-05-06：**`if`**、**`journal_tool_metrics`**、**`agent_task.require_truthy`**；**`command.params.parse_json_stdout`**；模板引用扫描扩展到 **`params`**；CLI **`journal-stats`**；默认 turn 用声明式门控替代 **`skill_generator`** 硬编码。
- 2026-05-06：`command` / `tool_call` 节点 **真正实现**：shell（等同 **`exec` 策略**）与 **`ToolRegistry` 单次工具调用**；`retrieve_context` 仍为透传。
- 2026-05-06：§1 `meta` 补充 **`transcript_mode: summary`**（及 `transcript_summary` 别名）、**`user_prompt_prefix` / `user_prompt_suffix`**；§3 **`$runtime.post_turn.ctx`**（并澄清 **`$runtime.run_journal.current_turn_metadata`** 为兼容占位）；§4 **`structured_memory_extract`**、`on_respond` 说明；§5 默认模板 PostTurn 输入改用 **`$runtime.post_turn.ctx`**。
- 2026-05-05：切换到 v2（nodes/depends_on/string-flow），移除 v1 `graph.edges` 主模型；新增 §8（`ResolveWorkflowPath`、`config.catalog`、与实现对齐），原修订记录顺延为 §9；**独立 `manifest.yaml` 已删除**，默认 Agent / 默认 turn 回落并入 **`config.yaml` → `catalog:`**。
