# 术语表（设计文档套件内）

下列术语在 [README.md](README.md)、[reference-architecture.md](reference-architecture.md)、[eino-md-chain-architecture.md](eino-md-chain-architecture.md)、[requirements.md](requirements.md)、[harness-governance-extensions.md](harness-governance-extensions.md) 中交叉出现，含义在此统一。**不绑定 Go 包名**；实现可用不同模块名，但语义建议保持一致。

| 术语 | 含义 |
|------|------|
| **Claw** | 产品线/范式：Agent 运行时 + 工具 + 渠道，把用户意图做成可重复自动化（对话或集成界面交付结果）。 |
| **clawbridge** | 多渠道入站/出站 **Go 模块** [`github.com/lengzhao/clawbridge`](https://github.com/lengzhao/clawbridge)：`InboundMessage`、Bus、drivers 等与运行时对接。 |
| **UserDataRoot** | 用户数据根目录（默认 **`~/.claw`** 或配置覆盖）：配置、转写索引、定时任务文件、全局 `.agent/` 与 workspace 等的锚点。 |
| **InstructionRoot** | `AGENT.md` 与记忆入口（`MEMORY.md` 或 `memory/` 分片）**同一套根路径**；是否与会话目录重合由「会话隔离」策略决定（见 [appendix-data-layout.md](appendix-data-layout.md)）。 |
| **SessionHandle** | 区分并发会话的键：通常 = 渠道实例 ID + 会话键（线程/话题 ID 等）。 |
| **`session_id`** | 持久化与会话目录命名用到的稳定 id（如 `SessionRoot` 段）；与 SessionHandle 的映射关系由宿主定义（需在 TurnContext / 落盘路径中一致）。 |
| **TurnHub** | 会话级邮箱/协调器：**同 SessionHandle 内**入站排队与策略（serial / insert / preempt）。 |
| **Engine** | 单轮（或单次 Submit）编排对象：装配 prompt、调用 TurnRunner、落盘。**推荐策略：每条入站任务新建 Engine，回合结束丢弃**，避免共享可变状态。 |
| **SubmitUser** | 处理一轮用户输入的业务入口（装配 → 模型/工具循环 → 成功后 transcript/dialog）。 |
| **TurnRunner** | 执行模型↔工具循环的组件；主路径为 **Eino ADK**。 |
| **Eino ADK** | CloudWeGo Eino 的 Agent 开发套件（`ChatModelAgent`、Runner、Middleware 等）。 |
| **PushRuntime** | 配置合并后写入进程内快照（如 `rtopts`），避免配置包与循环/预算包循环依赖。 |
| **ToolContext / TurnInbound** | 每回合注入工具可见的会话与入站元数据（正文通常单独走 messages，不合并进 TurnInbound 全文）。 |
| **Catalog（agents）** | 从 `agents/*.md` 加载的多 Agent 定义表：`agent_type` → frontmatter + system 正文。 |
| **Workflow（回合级）** | 用户声明的 **DAG**（`workflows/*.yaml`），编译为 Eino **Compose Graph**；线性 `steps` 仅为语法糖。**默认**：存在 **`workflows/<agent_type>.yaml`** 则该 Agent 使用之，否则用 manifest **`default_turn`**；frontmatter **`workflow:`** 可显式覆盖。见 [workflows-spec.md](workflows-spec.md) §3。 |
| **Registry（tools）** | 工具名 → 实现 + schema；可对子 Agent **过滤**得到子集。 |
| **Harness（执行束具）** | 模型外的运行时层：上下文拼装、工具编排、状态与预算、校验与审计等；扩展与治理见 [harness-governance-extensions.md](harness-governance-extensions.md)。 |
| **合成入站** | 定时器等非人类来源构造与人工消息同形的 Inbound，走同一 Submit 路径。 |
| **子 Agent 默认隔离** | **会话隔离**（派生 SessionRoot / subs 命名空间）+ **上下文隔离**（独立 messages，不默认注入父 MEMORY）；放宽须 frontmatter 显式开关。见 [appendix-data-layout.md](appendix-data-layout.md) §3.1、[eino-md-chain-architecture.md](eino-md-chain-architecture.md) §5.4。 |
| **write_behavior_policy** | Harness 策略：工具与演进写入允许的 **路径前缀、文件类型、大小、是否必须 staging、晋升条件**；与 FR-SKL-03、FR-MEM-03 及 [harness-governance-extensions.md](harness-governance-extensions.md) 的 policy 挂钩同一抽象。 |
| **Agent 执行记录** | 单次 ADK 运行的磁盘可追溯条目（如 JSONL）：`agent_type`、`session_id`、`run_id`、时间与 transcript 锚点等；主对话、记忆抽取、Skills 生成 **各自落盘**。 |
| **`workspace: shared` / `private`** | 工具默认 cwd：**shared** = 与当前主 Agent 回合同一工作目录；**private** = 独占目录，避免与主会话文件工具互扰。见 FR-AGT-06、[eino-md-chain-architecture.md](eino-md-chain-architecture.md) §5.2。 |
| **`memory_agent` / `skill_agent`（约定）** | Workflow **节点 id** 命名习惯：在 **`on_respond` 之后**挂 **`use: agent`** 且 **`async: true`**，默认 **`agent_type`** 为 **`memory_extractor`**、**`skill_generator`**（内置 Catalog，可被 **`agents/`** 覆盖）。见 [workflows-spec.md](workflows-spec.md) §4.3、§8。 |
