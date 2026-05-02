# 术语表（`docs/new_design` 套件内）

下列术语在 [README.md](README.md)、[reference-from-oneclaw.md](reference-from-oneclaw.md)、[eino-md-chain-architecture.md](eino-md-chain-architecture.md)、[requirements.md](requirements.md) 中交叉出现，含义在此统一。**不绑定 Go 包名**；新项目可用不同实现名称，但语义建议保持一致。

| 术语 | 含义 |
|------|------|
| **Claw** | 产品线/范式：Agent 运行时 + 工具 + 渠道，把用户意图做成可重复自动化（对话或集成界面交付结果）。 |
| **oneclaw** | 上述范式的一种 **Go 参考实现**（本仓库）；复制 `new_design` 到其他项目时，可将 oneclaw 仅当作对照物。 |
| **clawbridge** | oneclaw 默认接入的 IM/渠道框架（`InboundMessage` / Bus / drivers）；新项目可替换为自研 HTTP/Webhook，但建议保留「统一入站消息形状」的概念。 |
| **UserDataRoot** | 用户数据根目录（oneclaw 默认 `~/.oneclaw`）：配置、转写索引、定时任务文件、全局 workspace 等的锚点。 |
| **InstructionRoot** | `AGENT.md` 与 `MEMORY.md` **必须共处的目录**；是否与会话目录重合由「会话隔离」策略决定（见 [appendix-data-layout.md](appendix-data-layout.md)）。 |
| **SessionHandle** | 区分并发会话的键：通常 = 渠道实例 ID + 会话键（线程/话题 ID 等）。 |
| **TurnHub** | 会话级邮箱/协调器：**同 SessionHandle 内**入站排队与策略（serial / insert / preempt）。 |
| **Engine** | 单轮（或单次 Submit）编排对象：装配 prompt、调用 TurnRunner、落盘。**oneclaw 策略：每条入站任务新建 Engine，回合结束丢弃**。 |
| **SubmitUser** | 处理一轮用户输入的业务入口（装配 → 模型/工具循环 → 成功后 transcript/dialog）。 |
| **TurnRunner** | 执行模型↔工具循环的组件；oneclaw 主路径为 **Eino ADK**。 |
| **Eino ADK** | CloudWeGo Eino 的 Agent 开发套件（`ChatModelAgent`、Runner、Middleware 等）。 |
| **PushRuntime** | 配置合并后写入进程内快照（如 `rtopts`），避免配置包与循环/预算包循环依赖。 |
| **ToolContext / TurnInbound** | 每回合注入工具可见的会话与入站元数据（正文通常单独走 messages，不合并进 TurnInbound 全文）。 |
| **Catalog（agents）** | 从 `agents/*.md` 加载的多 Agent 定义表：`agent_type` → frontmatter + system 正文。 |
| **Registry（tools）** | 工具名 → 实现 + schema；可对子 Agent **过滤**得到子集。 |
| **合成入站** | 定时器等非人类来源构造与人工消息同形的 Inbound，走同一 Submit 路径。 |
