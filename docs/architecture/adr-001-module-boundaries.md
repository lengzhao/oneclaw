# ADR-001：claw 模块边界与接口形态

## 状态

已采纳。能力边界与默认方向需同时符合：

- [范围与非目标](../concepts/scope-and-non-goals.md)
- [默认自进化能力](../concepts/default-evolution.md)

## 背景

在 `oneclaw` 的定位里，内核负责提供可嵌入的 Agent 运行时基础设施，而不是直接承担完整产品控制面。因此需要将 **Ingress / 宿主 / 运行时 / 记忆 / 工具 / 观测** 解耦，便于后续接入 MCP、多通道、调度与渐进式多 Agent 编排，同时不被外部产品协议反向绑定。

## 决策

### 1. Go 模块与包布局

- **模块路径**：`github.com/lengzhao/oneclaw`（根目录 `go.mod`）。
- **不使用** `internal/` 目录（仓库约定）。
- **日志**：统一使用标准库 `log/slog`。
- **包划分**（按依赖方向从外到内）：

| 包名 | 职责 |
|------|------|
| `bus` | `InboundEvent` / `OutboundEvent`、发布订阅与按会话串行投递 |
| `router` | 将入口事件映射到 `session.SessionID` 与可选元数据 |
| `session` | `Session`、`Store`（会话元数据，非对话正文） |
| `memory` | `ConversationStore`（对话轮次持久化，可替换为 DB） |
| `llm` | `Provider`、`Message`、`ToolCall`、工具 schema 描述 |
| `tools` | `Tool`、`Registry`、执行与 JSON Schema 参数 |
| `skills` | `Skill`、`Registry`：可复用说明文本，合并进 system 块 |
| `middleware` | 围绕一次 `Loop.Run` 的 `Context` 链式切面 |
| `agent` | `Loop`：ReAct 迭代，组合上述接口 |
| `channel` | `Ingress`：外部入口适配器，统一写入 `bus` |
| `scheduler` | 定时与 HTTP Webhook 产生与用户消息同形的 `InboundEvent` |
| `host` | 组合 `router` + `bus` + `agent.Loop` 的默认宿主 |
| `workspace` | 从工作区目录加载 Markdown，并拼入 `Loop.SystemPrompt` 前缀 |

**运行时配置**：`cmd/oneclaw` 可选加载 `oneclaw.json`，见 [配置参考](../reference/config-reference.md)。配置用于 OpenAI、MCP stdio、会话压缩与后台分析等运行时行为，不改变包级边界。

### 2. 核心数据模型

- **`session.SessionID`**：字符串别名；全局唯一，由 `router` 根据通道+对等端生成或由调用方指定。
- **`bus.InboundEvent`**：`Ctx`（可选，非序列化；有则总线用其调用 `Handler`，便于继承 HTTP 取消/超时）、`ID`、`TraceID`、`SessionID`、`Source`、`Text`、`Metadata`。
- **`bus.OutboundEvent`**：一轮 Agent 成功结束后的规范出站单元（`TraceID`、`SessionID`、`Source`、`Text`、`Metadata`、`InboundID`）；具体投递由宿主定义。
- **`middleware.Context`**：绑定 `TraceID`、`SessionID`、本轮 `[]llm.Message`、`UserText`、`Result`、`Metadata` 浅拷贝，以及可选 `Inbound *bus.InboundEvent`。
- **`skills.Skill`**：`Name`、`Description`、`Content`；`LoadDir` 加载目录下 `*.md`。
- **`llm.Message`**：`Role`、`Content`、`ToolCallID`、`Name`。
- **`tools.Tool`**：`Name`、`Description`、`Parameters`、`Handle(ctx, json.RawMessage) (string, error)`。

### 3. 并发模型

- **会话隔离**：同一 `SessionID` 的事件进入该会话专用缓冲 channel，由单 goroutine 顺序执行 `agent.Loop.Run`。
- **多会话并行**：不同 `SessionID` 彼此并行。
- **总线入队**：`bus.Bus.Publish` 非阻塞优先；若会话队列满则返回 `ErrBackpressure`。

这一设计保证“默认多 Agent 但内核边界仍清晰”的心智模型成立，也为宿主侧编排、角色路由和子会话扩展保留空间。

### 4. MCP 与静态工具

- **首版**：`tools.Registry` 支持静态注册。
- **扩展方向**：预留 `MCPToolSource`（`ListTools` / `CallTool`）供后续接入 MCP 客户端，与 `Registry` 组合使用。
- **边界**：工具由模型显式 JSON 调用；中间件在固定阶段介入，不暴露给模型。

### 5. Skills 与工具边界

- **Skills**：静态注册表 + 可选目录加载；由 `agent.Loop` 在构造 system 消息时按策略拼接。
- **选择规则**：若 `Metadata` 含键 `skills`，则仅使用其逗号分隔名称列表；若无该键，则使用 `Loop.DefaultSkillNames`。
- **工具**：由 `tools.Registry` / 未来 MCP 提供 JSON Schema，模型通过 `tool_calls` 触发。

这一边界与“默认自进化”方向兼容：Skills 与工作区文档可作为低成本、外部化的可进化载体，而不需要把策略埋进代码路径。

### 6. 可观测性

- **`TraceID`**：由 Ingress 或 `scheduler` 生成 UUID，经 `middleware.Context` / 日志传入 `agent.Loop`。
- **日志实践**：在 `slog` 中优先使用 `slog.String("trace_id", ...)` 等 Attr，便于结构化采集。

### 7. Agent 入口

- **`Loop.Run(ctx, RunInput)`**：唯一入口。`RunInput` 含 `TraceID`、`SessionID`、`Text`、`Metadata`；可选 `Inbound *bus.InboundEvent`。
- **`host.Host.OnOutbound`**：可选；成功生成助手正文后构造 `bus.OutboundEvent` 并回调，再写默认 `slog` 回复日志。
- **`MaxIterations`**：`<= 0` 时本轮使用默认值（40），不写回 `Loop` 字段，避免并发共享的数据竞争。

### 8. 与 PicoClaw 工作区 Markdown 的概念对齐

- **目标**：不追求 OpenClaw Gateway 协议兼容，但允许在单 Agent 规则组织上复用固定文件名习惯，便于迁移和对照。
- **默认文件名**：`IDENTITY.md`、`SOUL.md`、`AGENTS.md`、`USER.md`，以及独立的 `skills/`。
- **oneclaw 首版实现**（`workspace` 包 + `cmd/oneclaw -workspace DIR`）：
  - 读取并拼入 system 前缀，顺序为 `IDENTITY` → `SOUL` → `AGENTS` → `USER`。
  - `AGENTS.md` 优先；若不存在或为空，则回退 `AGENT.md`。
  - `skills/` 目录由现有 `skills` 包与 `Loop.Skills` 处理。
  - `HEARTBEAT.md`、`TOOLS.md`、`MEMORY.md` 默认不自动加载，避免混淆长期记忆、调度提示与工具 schema 语义。

推荐目录职责与落点组织见 [工作区布局](../reference/workspace-layout.md)。

## 后果

### 优点

- 替换 `llm.Provider` / `memory.ConversationStore` 即可切换厂商与存储。
- 新通道只需实现 `channel.Ingress` 并 `Publish` 到 `bus`。
- Cron/Webhook 与 CLI 共用同一 `InboundEvent` 路径。
- 为默认自进化、默认多 Agent 分工与宿主侧编排保留了扩展空间。

### 代价

- `agent.Loop` 与 `bus` 解耦后，需要在 `cmd` 或宿主服务中显式组装依赖。
- 更重的路由、策略、审批与产品控制面不能靠内核“顺手做掉”，需由宿主承担。

## 参考

- [文档入口](../README.md)
- [默认自进化能力](../concepts/default-evolution.md)
- [Agent Profile 与任务路由](../concepts/agent-profiles-and-routing.md)
- [范围与非目标](../concepts/scope-and-non-goals.md)
- [配置参考](../reference/config-reference.md)
- [工作区布局](../reference/workspace-layout.md)
- [运维速查与排错](../reference/runbook-troubleshooting.md)
- [ADR-002：写回治理与 `WriteIntent` 流水线](./adr-002-write-governance.md)
- [ADR-003：任务编排信封与类型化元数据](./adr-003-orchestration-envelope.md)
- [ADR-004：上下文装配流水线](./adr-004-context-assembly-pipeline.md)
- [ADR-005：工作负载分级与队列优先级](./adr-005-workload-classes-and-priority.md)
- [路线图与长期方向](../notes/roadmap.md)
- [OpenClaw 能力对照与启示](../notes/openclaw-capabilities-and-design-notes.md)
