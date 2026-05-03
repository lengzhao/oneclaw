# `workflows/*.yaml` 配置规格（初版）

本文定义 Claw **回合级编排工作流** 的 YAML 形状：以 **有向无环图（DAG）** 为主模型（节点 + 边），与 Eino **Compose Graph** 对齐；线性 **`steps`** 仅作**语法糖**（展开为链状图）。满足 [requirements.md](requirements.md) FR-FLOW-02，并与 [eino-md-chain-architecture.md](eino-md-chain-architecture.md)、[architecture.md](architecture.md) §2.1 一致。**记忆抽取、Skills 生成等演进能力一律在图中声明**（典型：`on_respond` → **`async: true`** 的 **`use: agent`** 枝），**不**使用 Agent Catalog 上的演进开关字段。

**命名**：采用 **workflow** 而非 *chain*，强调 **分支、汇合与异步枝叶**；避免「必须是纯链式」的误解。（旧文档中的 `chains-spec` / `chains/` 已废弃，统一以本文为真源。）

**非目标**：本文 **不** 绑定 Eino 仓库内置 Graph YAML 方言；宿主将本规格 **编译** 为 `compose.Graph` / `Runnable`。

---

## 1. 文件位置与命名

| 规则 | 说明 |
|------|------|
| **默认目录** | `UserDataRoot/.agent/workflows/` |
| **扩展名** | `*.yaml` / `*.yml` |
| **与 `agent_type` 对齐（推荐）** | 若存在 **`workflows/<agent_type>.yaml`**（或 `.yml`），则 **该 Agent 默认使用此文件**；文件内顶层 **`id` 应与 `agent_type` 一致**（宿主可对「文件名 ≠ `id`」告警或拒载，策略文档化即可） |
| **其他 id** | 如 `default.turn`（默认回合）、共用模板等，文件名不必等于某 `agent_type`，由 manifest 的 `default_turn` 或 frontmatter **`workflow:`** 引用 |
| **discover** | 宿主扫描目录；**具体用哪条** 由 [§3 解析优先级](#3-与-manifest-的衔接及解析优先级) 决定 |

---

## 2. 顶层结构

### 2.1 必选字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `workflow_spec_version` | `int` | **规格版本**，当前 **`1`**；未知版本 **拒绝加载** |
| `id` | `string` | 稳定 id（日志、manifest、`runs/`）；建议 **`[a-z0-9._-]+`**。若文件名为 **`workflows/<agent_type>.yaml`**，**建议 `id` 与 `agent_type` 相同**，便于排查与校验 |
| **`graph`** | `object` | **主模型**：含 `entry`、`nodes`、`edges`（见 §4） |

### 2.2 可选字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `description` | `string` | 人类可读说明 |
| `defaults` | `map` | 各节点可继承的默认 `params`（浅合并；节点内 `params` 优先） |
| `meta` | `map` | 任意 kv；宿主 **忽略未知键** |
| **`steps`** | `array` | **语法糖**：与 `graph` **二选一**；若二者皆存，**以 `graph` 为准**（或加载报错，宿主选一种策略并文档化） |

### 2.3 Graph 最小示例

```yaml
workflow_spec_version: 1
id: minimal
description: PreTurn → 主 ADK → OnRespond
graph:
  entry: receive
  nodes:
    receive:
      use: on_receive
    prompt:
      use: load_prompt_md
    memory:
      use: load_memory_snapshot
    tools:
      use: filter_tools
    main:
      use: adk_main
    respond:
      use: on_respond
  edges:
    - { from: receive, to: prompt }
    - { from: prompt, to: memory }
    - { from: memory, to: tools }
    - { from: tools, to: main }
    - { from: main, to: respond }
```

---

## 3. 与 manifest 的衔接及解析优先级

建议在 **`UserDataRoot/.agent/manifest.yaml`**：

```yaml
workflows:
  # 无 per-agent 文件、且无 frontmatter 覆盖时使用的默认回合工作流
  default_turn: default.turn
  # 解析为 .agent/workflows/default.turn.yaml
```

### 3.1 本轮使用哪条 workflow（优先级从高到低）

1. **Agent frontmatter 显式指定**：**`workflow: <id>`**（可选兼容别名 **`chain: <id>`**，与 `workflow` 同义）→ 按 `<id>` 解析文件（见下）。用于共用一条图、或临时指向非约定文件名。
2. **约定文件**：若 **未**指定 frontmatter `workflow`/`chain`，且存在 **`.agent/workflows/<agent_type>.yaml`**（再尝试 **`.yml`**），其中 **`agent_type` 为当前回合 Catalog 中的类型 id** → **使用该文件**。
3. **回落默认**：否则使用 **`workflows.default_turn`**（manifest）。

由此：**多数 Agent 只需** `agents/<…>.md` 里的 `agent_type`，并在 **`workflows/` 下放同名 `*.yaml`** 即可专用编排；未单独配置的全部走 **`default_turn`**。

### 3.2 标识符 `<id>` → 磁盘路径

1. 标识符 → `.agent/workflows/<id>.yaml`（再尝试 `.yml`）。
2. 相对 `UserDataRoot` 的路径 → 按路径加载（若项目启用）。
3. **禁止** 默认解析到 `UserDataRoot` 之外的路径。

---

## 4. `graph` 对象

### 4.1 字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `entry` | `string` | **唯一入口节点 id**；执行从此节点开始 |
| `nodes` | `map` | `node_id` → [节点定义](#5-节点定义) |
| `edges` | `array` | 有向边列表；`from` / `to` 必须存在于 `nodes` |

### 4.2 边对象 `edges[]`

| 字段 | 类型 | 说明 |
|------|------|------|
| `from` | `string` | 源节点 id |
| `to` | `string` | 目标节点 id |
| `branch` | `bool` 或 `"true"` / `"false"` | **仅**当 `from` 为 **`use: if`** 的节点时允许：表示该边属于 **真枝** / **假枝**（见 §6.1）。其它边上的 `branch` **忽略或报错**（宿主选一种并文档化） |
| `when` | `string` | **预留**：通用边条件（非 `if` 专用）；v1 可不实现 |

**语义**：

- 图须为 **DAG**（宿主加载时 **检测环**，发现则报错）。
- **拓扑执行**：当节点所有 **入边来源** 均已完成，该节点 **就绪**。
- **汇合**：多入边 → 单节点，表示 join（全部前驱完成后再执行）。**例外**：从 **`if`** 出发的两枝 **运行时二选一**，未选中的枝 **整段子图不调度**；为避免与「全部前驱 join」冲突，见 §6.1 **静态校验**。

### 4.3 异步与并行（推荐约定）

在 **节点** 上使用 `async: true`（见 §5）：节点就绪后 **立即调度执行**，**不阻塞**同一前驱下 **其他就绪分支** 的调度（用于 `on_respond` 之后多条演进枝叶并行后台跑）。

**记忆抽取 / Skills 生成（推荐写法）**：在主路径 `adk_main → on_respond` 完成后，从 **`on_respond`** 拉出 **`async: true`** 的 **`use: agent`** 节点即可；节点 id 建议约定为可读名字（如 **`memory_agent`**、**`skill_agent`**），`params.agent_type` 指向 Catalog 中专用的 `memory_extractor`、`skill_generator` 等。**是否需要跑演进枝** 用 **`use: if`** + `params.expr`（或宿主提供的全局/会话 flag）控制，**不要**依赖 Agent md 上的演进布尔字段。

> 若仅需「先回复用户再后台演进」，典型形状为：`main → respond`，再由 `respond` 同时连到 **`memory_agent`** 与 **`skill_agent`**（均为 **`use: agent`** + **`async: true`**）。完整示例见 §8。

---

## 5. 节点定义

`nodes.<id>` 为映射：

### 5.1 必选

| 字段 | 类型 | 说明 |
|------|------|------|
| `use` | `string` | 内置或插件节点类型 id |

### 5.2 可选

| 字段 | 类型 | 说明 |
|------|------|------|
| `params` | `map` | 传入节点工厂的参数 |
| `async` | `bool` | 默认 **`false`**；`true` 时调度语义见 §4.3 |

条件分支 **不要**用未定义的节点自由字段；请使用内置 **`use: if`**（§6.1）。

内置 `use` 表与行为与旧 **chains** 草案一致，见 [§6 内置节点类型（v1）](#6-内置节点类型-v1)。

---

## 6. 内置节点类型（v1）

| `use` | 职责 | 典型 `params` |
|-------|------|----------------|
| `on_receive` | 校验、脱敏、附件、TurnContext | `max_attachment_mb` |
| `load_prompt_md` | 装载分段 md | `fragments: [...]` |
| `load_memory_snapshot` | memory 包 `LoadSnapshot` | `respect_omit_memory_injection: true` |
| `filter_tools` | Registry 过滤 | `allowlist_ref: ...` |
| `adk_main` | 主对话 ADK | `agent_from_context: true` 或 `agent_type` |
| `if` | **条件分支**：求值后 **只沿一条出边** 继续调度 | 见 §6.1（如 **`expr`**；表达式语言由宿主定义，须沙箱化） |
| `noop` | **空节点**：立即完成，无副用 | 常用于 `if` 的假枝收口 |
| `agent` | **独立一次 ADK 运行**（子 Agent、**回合后**记忆/Skills 演进、其它后台管线）。**后台记忆/Skills**：在该节点上设 **`async: true`**，并典型命名为 **`memory_agent` / `skill_agent`**（仅为 id 约定，参见 §4.3） | **`agent_type`**（必填，Catalog id）, `workspace`, … |
| `memory_extract_llm` | 事实抽取（可用 **`use: agent` + `async`** 等价替代；本节点为可选捷径） | `staging_only: true` |
| `skill_suggest_llm` | Skills 草案（同上） | `staging_only: true` |
| `on_respond` | 裁剪、transcript、`runs`、Bus | `stream: true` |
| `retrieve_context` | （可选 RAG） | `backend_ref`, `top_k` |
| `command` | 外部命令（policy） | `argv`, `timeout_sec` |

### 6.1 内置 `if` 分支（v1）

- **形状**：`nodes.<id>.use == "if"` 的节点必须有 **恰好两条**出边，且分别带 **`branch: true`** 与 **`branch: false`**（布尔或等价字符串）。
- **语义**：`if` 就绪后，宿主对 **`params`**（如 **`expr`**）求值，得到布尔；**仅**调度匹配分支上的 **`to` 节点**；另一分支的 **整个下游子图** 标记为 **跳过**（不执行、不参与 join 计数）。
- **静态校验（推荐必做）**：从 **真枝** / **假枝** 各自出发，沿 **有向边** 可达的节点集合（不含 `if` 自身）**不得相交**。否则下游若存在汇合节点，易出现「一侧从未执行却等待 join」的歧义；若将来支持 **OR-join**，再在规格中单独定义。
- **假枝无工作**：可接到 **`noop`**，再接后续公共节点 **违反**上条不相交规则时，应改为把公共收尾 **复制**到两枝或重构图。

**示例**（仅全局允许演进时跑记忆 Agent）：

```yaml
nodes:
  gate:
    use: if
    params:
      expr: "context.flags.evolution_enabled"
  memory_agent:
    use: agent
    async: true
    params: { agent_type: memory_extractor }
  skip_mem: { use: noop }
edges:
  - { from: respond, to: gate }
  - { from: gate, to: memory_agent, branch: true }
  - { from: gate, to: skip_mem, branch: false }
```

---

**是否只要「最后多一个 Agent、且异步」？** **可以。** 主路径仍是 `… → on_respond`；在图上从 **`on_respond` 再引一条（或多条）出边** 到 **`use: agent`** 的节点，并在该节点上设 **`async: true`**（常见节点 id：**`memory_agent`**、**`skill_agent`**），即表示 **用户可见回复已完成后**，再 **后台** 跑专用 `agent_type`。**不必**把记忆抽取塞进 `adk_main` 同一次 ReAct 里。需要 **开关演进** 时用 **`if`**（§6.1）或配置 flag，表达式见宿主约定。

**与 oneclaw 实现对齐**：默认 **`workflows/default.turn.yaml`**（及模板）在 **`on_respond` 之后**串联 **`memory_agent` / `skill_agent`**（`async: true`，`agent_type` 分别为 **`memory_extractor` / `skill_generator`**）。二者为 **嵌入内置 Catalog**，用户可在 **`agents/`** 下放同名 md **覆盖**。子 Agent 与普通 **`use: agent`** 节点相同，**无**单独的演进类型配置或 `TurnContext` 嵌套剖面。**未**实现「演进专用 workflow 不得再挂同类 async 枝」的加载期校验；避免闭环依赖编排约定与后续可选护栏。

---

## 7. `steps` 语法糖（线性图）

当仅提供 `steps` 且不提供 `graph` 时，宿主 **展开**为：

- 生成节点 `step_0` … `step_{n-1}`，`use` / `params` / `async` 来自对应 step。
- 生成边 `step_i → step_{i+1}`。
- **`entry`** = `step_0`。

`steps` 元素字段与旧链式规格相同：`use`（必选）、`id`（可选覆盖自动 id）、`params`、`async`。

```yaml
workflow_spec_version: 1
id: linear-only
steps:
  - use: load_prompt_md
  - use: adk_main
  - use: on_respond
```

等价于一条链状 DAG。

> **注意**：纯 `steps` **只有单链**，**不能**表达「`on_respond` 已完成后、再分岔出 **并行异步** 的 Agent 枝」。若需要 **先回复再后台跑记忆/Skills**（或别的 **`agent`** 节点），请使用 **`graph`**（§8）。

---

## 8. 推荐示例：主路径 + 异步双枝演进

```yaml
workflow_spec_version: 1
id: default.with-evolution
graph:
  entry: recv
  nodes:
    recv: { use: on_receive }
    prompt: { use: load_prompt_md }
    memsnap: { use: load_memory_snapshot }
    ftools: { use: filter_tools }
    main: { use: adk_main }
    respond: { use: on_respond }
    memory_agent:
      use: agent
      async: true
      params: { agent_type: memory_extractor }
    skill_agent:
      use: agent
      async: true
      params: { agent_type: skill_generator }
  edges:
    - { from: recv, to: prompt }
    - { from: prompt, to: memsnap }
    - { from: memsnap, to: ftools }
    - { from: ftools, to: main }
    - { from: main, to: respond }
    - { from: respond, to: memory_agent }
    - { from: respond, to: skill_agent }
```

---

## 9. 用户扩展（插件）

- 宿主提供 **`RegisterWorkflowNode(use string, factory Factory)`**（命名可调整）。
- 未知 `use` → 加载期或运行期错误（策略由宿主定义）。

---

## 10. 与 Eino 的映射（实现提示）

| 概念 | Eino 侧 |
|------|---------|
| `graph.nodes` | `compose.NewGraph` + `AddLambdaNode` / `AddChatModelNode` / … |
| `graph.edges` | `AddEdge(from, to)`；入口连 `compose.START` → `entry` |
| `if` | 编译为 **分支 Lambda** 或 **两条条件边**；未选枝 **不** `AddEdge` 到 Compose（或运行时短路） |
| 叶子节点 | 连向 `compose.END`（或收口到统一出口节点） |
| 多枝 async | 叶子异步节点不阻塞 END；由宿主 Runnable 封装「先返回用户再后台跑」 |

---

## 11. 校验清单（加载时）

1. `workflow_spec_version == 1`。
2. `id` 非空；`graph.entry` 存在且 `graph.nodes[entry]` 存在。
3. 所有 `edges` 的 `from`/`to` 均在 `nodes` 中。
4. **无环**（DAG）。
5. **编排**：DAG / fan-out 等由 `workflow.Validate` 与宿主约定覆盖；**不**包含演进专用的额外静态规则（见 §6 说明）。
6. `steps` 与 `graph` 互斥或优先级符合宿主文档。
7. 每个 **`use: if`** 节点：**恰好两条**出边，`branch` 一真一假；真/假枝可达节点集 **不相交**（§6.1）。

---

## 12. 修订记录

| 日期 | 说明 |
|------|------|
| 2026-05-02 | 初版：workflow + **graph 主模型**、`steps` 糖、manifest `workflows`、`workflow` frontmatter；取代原 `chains-spec`/`.agent/chains` 命名；§1/§3：**解析优先级**（frontmatter → `workflows/<agent_type>.yaml` → `default_turn`）；推荐 **`id` 与 `agent_type` 一致**；§6/§7：内置 **`agent`** 节点；**`on_respond` 后异步枝** 需 `graph`；§4.2/§6.1：内置 **`if`** + 边 **`branch`**，**`noop`**；§11：`if` 静态校验 |
| 2026-05-03 | §4.3 / §6 / §8：**记忆抽取与 Skills 仅通过 workflow（`async` + `use: agent`）声明**；移除 Catalog 演进开关叙述；`if` 示例表达式不再引用 `evolution_suppressed`。**§6 / §11 与实现对齐**：内置 Catalog（`memory_extractor` / `skill_generator`）+ 默认 turn 模板；**无**演进专用加载期闭环校验、**无** `TurnContext` 演进嵌套剖面；`wfexec.Execute` 每次编译 DAG，`async` 节点 goroutine 触发且 handler 仍经 `ExecMu` 串行 |
