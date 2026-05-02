# 附录：用户数据根与 Workspace 布局（摘要）

返回：[README.md](README.md) · [requirements.md](requirements.md) §5

本文为 **`requirements.md` §5** 与 **[reference-architecture.md](reference-architecture.md)** 中路径概念的**自洽展开**。细节演进以实现为准。

**目录锚点（已定）**：**真源与可写数据默认落在用户主目录**，`UserDataRoot = ~/.<app>`（实现可配置，不得依赖「当前工作目录 / 项目根」作为默认真源）。仓库内业务代码、单测与样例可另用临时目录；产品行为以主目录树为准。

**主会话默认（推荐，与下节子 Agent 策略一致）**：**开启会话隔离**（`InstructionRoot = SessionRoot`），每会话一套 `AGENT.md` / 记忆 / `workspace/`，避免多通道多话题串味。需「全局共享一份 MEMORY」时显式关隔离，退化为 §2 扁平模式。

---

## 1. 术语（与 `glossary.md` 对齐）

- **UserDataRoot**：用户级根路径，**默认在用户主目录**（示例：`~/.claw`，以产品名为准）。
- **InstructionRoot**：**`AGENT.md` 与记忆入口必须共处于同一 InstructionRoot**。「记忆入口」可为 **`MEMORY.md`**，或 **`memory/` 目录下的分片文件**（及 sidecar）；二者不应与 `AGENT.md` 分属不同根路径。
- **Workspace**：默认作为文件工具 / `exec` 等工作目录的路径；通常为 `<InstructionRoot>/workspace`（目录名固定为 `workspace`）。
- **SessionRoot**：`UserDataRoot/sessions/<session_id>/`，承载该会话的转写等；是否兼作 InstructionRoot 由 **会话隔离** 策略决定。

---

## 2. 未开启会话隔离（扁平）

- **InstructionRoot = UserDataRoot**。
- 典型包含：`config.yaml`、`AGENT.md`、`MEMORY.md`（或 `memory/` 分片）、`rules/`、`workspace/`、`sessions/<id>/transcript*.json`、`scheduled_jobs.json`（位置以实现为准）。
- **与 PRD §5 对齐时**，同一用户数据根下还常有 **`agents/`、`skills/`、`workflows/`** 及 **Manifest**（路径可为 `.agent/manifest.yaml` 或配置声明的其他根）；完整列见 [requirements.md](requirements.md) §5，此处仅强调与 InstructionRoot 共位的「说明 + 记忆入口」。

---

## 3. 开启会话隔离（**主会话推荐默认**）

- **InstructionRoot = SessionRoot**（`UserDataRoot/sessions/<session_id>/`），其下仍应有配对的 `AGENT.md`、记忆入口（`MEMORY.md` 或 `memory/`）与同构的 `workspace/`。
- 全局 `UserDataRoot` 仍保留 **全局** `config.yaml`、**全局** `agents/`、`skills/`、`workflows/`、`.agent/manifest.yaml`（或等价路径）——**角色定义与 workflow 定义共享**；**每会话可变的是 InstructionRoot 内的说明、记忆与工作区**。

### 3.1 子 Agent（**默认：会话隔离 + 上下文隔离**）

已定策略：

- **上下文隔离**：子 Agent **默认**使用 **独立的 ADK 消息列表**；**不**把主会话完整 transcript 注入子循环；**不**注入主会话的 `MEMORY` / `memory/`（除非该 Agent 声明 **`inherit_parent_memory: true`** 等显式开关）。
- **会话隔离**：子 Agent 若有独立落盘需求（子 transcript、子演进写入、**按 Agent 的执行记录**），使用 **派生命名空间**，例如 `sessions/<parent_session_id>/subs/<sub_run_id>/`（或以内存为主、仅在 handoff 时写回父会话摘要 —— 实现二选一，文档层约束「不得默认写进父 SessionRoot 同一 transcript 文件不打标签」）。
- **Workspace（工具 cwd）**：子 Agent / PostTurn 管线 Agent **默认 `shared`** —— 与 **当前主 Agent 回合** 相同的工作目录（一般为会话 `<InstructionRoot>/workspace`）；若声明 **`workspace: private`**，使用独占子目录（常与 `subs/<sub_run>/workspace` 对齐），避免文件/exec 与主会话互相干扰。
- **演进防递归**：承担 **记忆抽取** / **Skills 生成** 的 Agent 必须 **`suppress_post_turn_evolution: true`**，宿主 **不得**在其运行结束后再次调度这两项 PostTurn（见 [requirements.md](requirements.md) FR-FLOW-05）。

可选放宽（均在 Agent frontmatter 或 manifest 中 **显式开启**）：`inherit_parent_memory`；合并摘要回父 transcript。

---

## 4. 设计意图（便于新项目取舍）

1. **配置与「干活目录」分离**：降低误删配置/记忆入口的风险。
2. **同会话内路径一致**：隔离模式下会话内相对布局与扁平模式「形状一致」，仅根路径从 UserDataRoot 换成 SessionRoot。
3. **数据根树内避免嵌套同名隐藏目录**：实现上常以固定子目录名（如 `workspace/`、`sessions/`）表达，而非多层 `.app/.app`。

---

## 5. 与 FR 的对应

| 需求文档中的概念 | 本附录位置 |
|------------------|------------|
| `sessions.isolate_workspace`、共享 vs 每会话 workspace | §2–§3 |
| IM 转写路径 under `sessions/<id>/` | §2–§3 |
| 子 Agent 默认隔离、Workspace 默认共享 | §3.1 |
| 演进防递归 | §3.1 |

---

## 6. 其余推荐默认（实现与运维，非强制性验收）

以下为与已定目录、子 Agent 策略 **配套** 的推荐方案，减少歧义与返工：

| 主题 | 推荐 |
|------|------|
| **`session_id`** | 由 **SessionHandle** 稳定派生（规范化字符串后哈希或安全 slug），保证同一 IM 线程/话题始终映射同一 `sessions/<id>/` |
| **PostTurn 演进** | **异步**为主；下一回合 PreTurn **best-effort** 读取上一轮已落盘 MEMORY（若未到盘则仅用当期上文）。需强一致时提供配置 **「_reply 前 flush 演进队列」**（延迟换一致性） |
| **演进写入** | 默认 **staging**（如 `memory/.staging/`、`skills/.staging/`）+ **write_behavior_policy** 晋升；高危路径禁止直写真源 |
| **Catalog 加载顺序** | **内置 agents → 用户 `UserDataRoot/.agent/agents/`**；同名 **用户覆盖** |
| **知识库原文** | 默认放在 **`UserDataRoot/knowledge/sources/`**（或 manifest 声明的绝对/相对 **UserDataRoot** 路径），与向量索引（可重建）分离 |
| **可观测** | 子 Agent subsession 的日志/trace **带 `parent_session_id` + `sub_run_id`**；**每个 `agent_type` 的执行记录**单独落盘（见 FR-AGT-05） |
