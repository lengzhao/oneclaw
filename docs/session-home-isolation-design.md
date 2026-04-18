# 用户根 + 按会话工作区隔离（Session Home）设计

本文描述一种**以 `~/.oneclaw` 为全局根**、**以 `~/.oneclaw/sessions/<session_id>/` 替换当前「项目 `-cwd`」** 作为运行时工作目录的模型，使多会话在**默认文件视图**上相互隔离；公共规则从用户根读取。与现状对照见 [config.md](config.md)、[runtime-flow.md](runtime-flow.md)。

**目录布局的推荐真源**（用户根与 `workspace/` 子目录、`AGENT.md` 与 `MEMORY.md` 同目录等）见 **[user-root-workspace-layout.md](user-root-workspace-layout.md)**；下文 §4 的树状示例在实现对齐该文档后可能演进，以 `user-root-workspace-layout` 为准。

---

## 1. 背景与动机

**现状（项目-centric）**：`cmd/oneclaw` 用 `-cwd` 解析出的**项目根**作为 `Engine.CWD` / `toolctx.CWD`。同一项目目录下多个 IM 会话共享同一棵 `<cwd>/.oneclaw/`（除 `sessions/<id>/` 下转写、审计等已按会话分片外），**exec 默认 pwd、tasks、session 级 memory 路径**仍以项目根为锚，会话之间容易在「相对路径与任务文件」层面交叉影响。

**目标**：把「会话可见、可写的默认文件宇宙」收束到**每会话独占目录**；**配置与全局 AGENT / rules** 仍集中在用户根，避免每个会话复制一份密钥与总规范。

---

## 2. 目标与非目标

### 2.1 目标

- **用户根（UserRoot）**：`~/.oneclaw`（或通过既有 `paths.memory_base` 解析后的等价基路径，下文默认 `~/.oneclaw`）。
- **公共只读（概念上）**：用户根下放置 **`AGENT.md`、`rules/`**（及可选 `skills/` 索引策略，见 §7），供所有会话的系统提示与策略引用。
- **会话根（SessionHome / InstructionRoot）**：`~/.oneclaw/sessions/<session_id>/` 作为**该会话的说明与规则记忆根**（`AGENT.md` / `MEMORY.md` 与全局布局一致），与 `session.StableSessionID(handle)` 对齐；**默认干活目录**为 **`<SessionHome>/workspace`**（即 `Engine.CWD`），与用户数据根树内**不出现嵌套目录名 `.oneclaw`** 的约定一致（见 [user-root-workspace-layout.md](user-root-workspace-layout.md)）。
- **默认隔离**：不同 `session_id` 下默认 **exec pwd、read/write 的 cwd 语义、tasks、本会话 `memory/` 等** 互不重叠（在工具策略允许范围内）。

### 2.2 非目标

- **不做 OS 级沙箱**：同一进程、同一用户下，仍可通过绝对路径、网络、子进程访问机内其他资源；隔离是**约定 + 路径锚点**，不是容器或 Seatbelt。
- **不自动改变 clawbridge / 渠道协议**：仅改变 oneclaw 侧 **cwd 与落盘根** 的解析方式。
- **不强制废弃「项目模式」**：可为 CLI `-cwd`、`-init`、单仓库开发保留 **可选** 项目层配置（见 §6），与常驻 IM 模式的「用户根 + SessionHome」并存或分入口声明。

---

## 3. 术语

| 术语 | 含义 |
|------|------|
| **UserRoot** | `memory.MemoryBaseDir(home)` 解析结果，默认 `~/.oneclaw` |
| **session_id** | 与现实现一致：`session.StableSessionID(SessionHandle)`（稳定、可哈希、用于目录名） |
| **SessionHome / InstructionRoot** | `filepath.Join(UserRoot, "sessions", session_id)`；**`Engine.CWD`** = `filepath.Join(SessionHome, "workspace")` |
| **ProjectRoot（旧）** | 当前 `cmd/oneclaw` 的 `-cwd`；本设计在 IM 主路径上**不再**作为工具默认 cwd |

---

## 4. 目录布局

### 4.1 用户根（共享）

```text
~/.oneclaw/
  config.yaml              # 与用户级合并逻辑一致（见 §6）
  AGENT.md                  # 全局默认 Agent 说明（可选；会话内可覆盖见 §7）
  rules/                    # 全局规则片段（可选）
  skills/                   # 若采用「用户级 skills 索引」策略（可选）
  sessions.sqlite           # 可选：仍可放在 UserRoot（推荐），与「会话数据在 sessions/ 下」一致
  sessions/
    <session_id>/
      ...                   # SessionHome：见 §4.2
```

### 4.2 会话根（隔离）

将 **SessionHome** 视为**小 `UserRoot`**：其下**不再**嵌套名为 `.oneclaw` 的子目录；`AGENT.md`、`MEMORY.md`、`memory/`、`tasks.json`、`audit/` 等与未隔离时的 `~/.oneclaw` **相对结构一致**。

```text
~/.oneclaw/sessions/<session_id>/
  AGENT.md                  # 可选：本会话覆盖/增量
  MEMORY.md                 # 与 AGENT.md 同目录
  rules/                    # 可选
  memory/                   # 日更 episodic 等
  tasks.json
  audit/
  transcript.json
  working_transcript.json
  workspace/                # Engine.CWD；exec、read/write 默认锚点
    media/inbound/          # 入站附件等（实现以代码为准）
```

**说明**：上表为逻辑布局；**transcript / sqlite / audit** 的精确路径应与 `config.Resolved` 的派生规则一次性对齐（§5），避免「配置算一套路径、Engine 另一套 cwd」长期分裂。按项目分片的自动记忆推荐落在 **`<memory_base>/projects/<slug>/`**（见 `memory.AutoMemoryDir`），**不必**在仓库内再建 `<repo>/.oneclaw/`；旧代码路径仍以 `memory.DefaultLayout` 为准，直至收敛。

---

## 5. 配置与路径解析（核心）

### 5.1 双根原则

引入显式概念（实现上可为 `Resolved` 扩展字段或并行参数）：

- **ConfigRoot / UserRoot**：加载 YAML、API key、MCP 静态配置等；默认仅 `~/.oneclaw` + 可选 `-config` 文件。
- **SessionWorkspace**：`SessionHome`，仅由 **`session_id`** 派生；**`MainEngineFactory` 创建 `Engine` 时写入 `Engine.CWD`**。

`config.SessionTranscriptPaths(session_id)`、`SessionsSQLitePath()` 等应基于 **同一套「数据根」** 计算：推荐 **UserRoot** 为会话索引根，例如：

- `sessions.sqlite` → `~/.oneclaw/sessions.sqlite`（或 YAML 覆盖）
- `transcript` → `~/.oneclaw/sessions/<session_id>/transcript.json`（与 SessionHome 同级，**不**再置于 `SessionHome/.oneclaw/`）

避免 transcript 仍在「旧项目 cwd」而工具已在 SessionHome 的**跨盘不一致**。

### 5.2 与 `PushRuntime` / `rtopts` 的关系

全局预算、维护开关等仍来自合并后的 YAML；**与 cwd 无关的路径**（如 `log.file`）应明确相对于 **UserRoot** 或绝对路径，而不再默认相对于已废弃的 ProjectRoot（IM 模式）。

```mermaid
flowchart LR
  subgraph load [启动加载]
    Y1["~/.oneclaw/config.yaml"]
    Y2["-config 文件"]
    Y1 --> Merge[合并 Resolved]
    Y2 --> Merge
    Merge --> RT[PushRuntime / rtopts]
  end
  subgraph job [每轮 Worker Job]
    H[SessionHandle] --> SID[StableSessionID]
    SID --> SH[SessionHome]
    SH --> ENG["NewEngine(CWD=SessionHome/workspace)"]
    Merge --> ENG
  end
```

---

## 6. 配置合并顺序（建议）

**IM 常驻模式（本设计主场景）**：

1. `~/.oneclaw/config.yaml`
2. 可选：`-config` 显式文件

**可选保留「项目覆盖」**（便于本地仓库仍放 `repo/.oneclaw/config.yaml`）：

3. 若存在 **显式 ProjectRoot**（环境变量或仅 CLI 入口）：再合并 `<ProjectRoot>/.oneclaw/config.yaml`，优先级高于用户根。

文档与实现需标明：**IM worker 路径默认不扫描 ProjectRoot**，以免隐式依赖当前 shell 的 cwd。

---

## 7. 系统提示与 AGENT / rules / skills

| 来源 | 路径 | 用途 |
|------|------|------|
| 全局 | `UserRoot/AGENT.md`、`UserRoot/rules/**` | 所有会话共享的基线 |
| 会话覆盖（可选） | `SessionHome/AGENT.md`、`SessionHome/rules/**` | 仅本会话；合并策略建议：**会话覆盖 > 全局** 或 **拼接（全局 + 会话增量）**，产品二选一并写死 |

**skills**：若索引扫描仍基于 `toolctx.CWD`（`workspace/`），则会话级 skills 可放在 `SessionHome/workspace/skills` 或实现约定的路径（**不在**用户数据根下再嵌套名为 `.oneclaw` 的目录）；若希望共享用户级 skills，需在 `session`/`prompt` 组装处 **额外注入 `UserRoot/...` 下既定目录**，与现 `skills.PromptSkillLines(cwd, home, …)` 行为对齐改造。

---

## 8. 工具、exec、附件

- **exec**：`cmd.Dir` 与 `tctx.CWD` 对齐（通常为 `SessionHome/workspace`）；run log 建议固定在 `SessionHome/exec_log/<ts>/run.log` 或实现约定路径（**不在** `SessionHome` 下再嵌套 `.oneclaw/`）。
- **read_file / write_file**：以现 cwd 策略为基础，自然限制在 SessionHome 树内（外加 `MemoryWriteRoots` 等例外）。
- **入站附件**：`PersistInlineAttachmentFiles`、`mediastore` 根应落在 **SessionHome** 下，避免多会话写入同一项目 `media/`。

---

## 9. memory 与 maintain

- **DefaultLayout(SessionHome, home)**： episodic、dialog_history 等应落在 **SessionHome** 或 **UserRoot 下按 session_id 分片**（与现 `dialog_history` 按日期 + session 分文件策略兼容，只需把「项目 slug」从「项目 cwd」改为「session_id」或固定前缀）。
- **maintainloop（定时维护）**：需定义维护对象：  
  - **A**：按会话轮询 SessionHome（重）；  
  - **B**：仅维护 UserRoot 下全局 memory + 最近活跃 session 列表（轻）；  
  - **C**：维护推迟到「该 session 回合结束」的 PostTurn（现 `RunPostTurnMaintain`），定时任务只跑全局。  

推荐首版 **B + C**，避免全盘扫描 `sessions/*`。

---

## 10. MCP、usage、export-session

- **MCP 注册**：stdio 子进程 cwd 或 artifact 目录应基于 **UserRoot** 或 **SessionHome** 明确其一；多会话并发时 artifact 建议 **`SessionHome/artifacts`**（或 `workspace/` 下子目录），避免文件名碰撞。
- **usage 落盘**：IM 下锚 **InstructionRoot**：`<InstructionRoot>/usage/`；全局统计可扫描 `sessions/*/usage/` 或统一到 UserRoot（任选，需一致）。
- **export-session**：源路径从「`<cwd>/.oneclaw`」改为「`SessionHome` 下扁平布局 + UserRoot 中与本 session 相关的条目标记」；或提供「仅导出 SessionHome」模式。

---

## 11. CLI 与迁移

| 场景 | 行为建议 |
|------|----------|
| `oneclaw -init` | 初始化 **UserRoot**（config 模板、全局 AGENT.md/rules）；**不**再要求项目目录（除非指定 ProjectRoot） |
| 首次某 `session_id` 收到消息 | `MkdirAll(SessionHome/...)`（含 `workspace/`），可拷贝最小模板（空 MEMORY.md、空 tasks） |
| 从旧部署迁移 | 脚本或文档：按 `StableSessionID` 将 `<旧cwd>/.oneclaw/sessions/<id>/*` 迁到 `~/.oneclaw/sessions/<id>/`；config 迁到 `~/.oneclaw/config.yaml` |

---

## 12. 风险与开放问题

1. **磁盘增长**：每会话一棵 `.oneclaw`；需保留策略、导出与手动清理文档。
2. **用户是否仍需要「绑定某 Git 仓库」**：若需要，可在 SessionHome 下 `git clone` 或由用户把 `SessionHome` 设为某工作副本根；本设计不内置「项目 = Git 根」绑定，除非另加元数据。
3. **StableSessionID 碰撞**：沿用现哈希长度与字符集；目录段需继续 sanitize（与现 `exec` 对 session 路径段一致）。
4. **子 Agent**：`toolctx.ChildContext` 继承同一 `SessionHome` 与父会话一致；若未来要子 Agent 独立 workspace，需再引入 `AgentWorkspace` 子目录。

---

## 13. 建议落地顺序

1. **路径层**：`Resolved` / `MainEngineFactory`：派生 `SessionHome`，`Engine.CWD = SessionHome`；**transcript / sqlite** 与 UserRoot 对齐。
2. **exec / mediastore / tasks**：验证相对路径与日志路径无歧义。
3. **prompt**：全局 AGENT/rules + 可选会话覆盖。
4. **maintain / export / docs**：更新 [config.md](config.md)、[runtime-flow.md](runtime-flow.md) 中的「会话与多通道」一节，标明 IM 模式默认数据根。

---

## 14. 小结

- **可以**用 `~/.oneclaw` 管配置与公共 `AGENT.md`/`rules`，用 `~/.oneclaw/sessions/<session_id>/` **替换**当前 IM 路径下的项目 `cwd`，从而在**默认工具与文件布局**上实现会话级隔离。
- 实现关键是 **ConfigRoot 与 SessionWorkspace 分离**，并 **统一 transcript、SQLite、审计、MCP artifact** 的锚点，避免半套旧「项目 cwd」半套新 SessionHome。
- **完全隔离**仅限文件与约定层面；安全边界仍需工具策略与运维模型配合。

---

## 15. 实现状态（`cmd/oneclaw` 常驻 IM）

已在主进程落地（`home` 非空的 `config.Load` 路径）：

- `config.Resolved.UserDataRoot()`、`SessionTranscriptPaths` / `SessionsSQLitePath` / 默认 media 等与用户数据根对齐。
- `MainEngineFactory`：`Engine.CWD` 由 **`sessions.isolate_workspace`** 决定（默认 **false**：`CWD = <UserDataRoot>/workspace`；**true**：`CWD = <UserDataRoot>/sessions/<StableSessionID>/workspace`）。`Engine.UserDataRoot` 供 cron / system 提示；用户数据根树内**不**再嵌套名为 `.oneclaw` 的目录（tasks、memory、audit 等锚 **InstructionRoot**，见 `memory.JoinSessionWorkspaceWithInstruction`）。
- `toolctx.HostDataRoot`：`schedule.Add/List/Remove` 写入 `<UserDataRoot>/scheduled_jobs.json`；`StartHostPollerIfEnabled` 使用同一根目录。
- 审计：`RegisterAuditSinks` 在 IM 下使用 **InstructionRoot** + `OmitDotDir`，路径为 `<InstructionRoot>/audit/…`。
- exec：`run.log` 位于实现约定路径（如 `<InstructionRoot>/exec_log/<ts>/`），**不**经 `SessionHome/.oneclaw/`。
- 定时维护：`maintainloop` 使用 `memory.IMHostMaintainLayout(UserDataRoot, home)`。

`config.Load` **仅**合并 `~/.oneclaw/config.yaml` 与可选 `-config`（相对路径相对 `~/.oneclaw/`），**不再**读取项目目录或进程 `cwd`。`-init` / `-export-session` / `-maintain-once` 均以 **`UserDataRoot()`**（默认 `~/.oneclaw`）为数据根。
