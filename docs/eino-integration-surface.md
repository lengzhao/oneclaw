# Claw 项目用到的 Eino 功能与接口清单

本文档**逐项列出**实现 Claw 运行时预期依赖的 **CloudWeGo Eino**（及 **eino-ext**）能力与典型 Go 符号，便于实现时对照 import 与版本迁移。架构设计与挂载方式仍以 [eino-md-chain-architecture.md](eino-md-chain-architecture.md)、[requirements.md](requirements.md) 为准。

**说明**：

- 下列 **import 路径**、**类型名** 以 Eino 当前公开文档与仓库为准；上游重命名时请以 [`github.com/cloudwego/eino`](https://github.com/cloudwego/eino) 与官方文档为准，可用 Context7 `/websites/cloudwego_io` 查询最新说明。
- **Memory / Session 持久化格式** 在 Eino 文档中多为**应用层责任**；Claw 的 `MEMORY.md`、`TurnContext`、执行记录落盘等 **不** 等同于 Eino 内置组件，见本文 §9。

---

## 1. Go 模块依赖

| 模块 | 用途 |
|------|------|
| `github.com/cloudwego/eino` | 核心：`schema`、`compose`、`components/*`、`adk`、`callbacks` 等 |
| `github.com/cloudwego/eino-ext` | 扩展实现：OpenAI/Ark 等 **ChatModel**、**Embedding**、**Indexer/Retriever**（Redis、ES 等）、**Document Transformer** 等 |

安装示例（文档常见要求 **stable ≥ v0.5.0**，Middleware 在 **v0.8** 起有接口演进，见 §8）：

```bash
go get github.com/cloudwego/eino@latest
# 按需
go get github.com/cloudwego/eino-ext@latest
```

---

## 2. `github.com/cloudwego/eino/schema` — 消息与文档模型

| 符号（典型） | Claw 中的用途 |
|--------------|----------------|
| `Message` | ADK 与 ChatModel 之间的多轮对话载体；`Role`、`Content`、`ToolCalls` 等 |
| `UserMessage` / `AssistantMessage` 等构造辅助 | 拼装 PreTurn 注入、Middleware 插入片段 |
| `ToolCall` / `FunctionCall` | 模型发起工具调用；与 ToolsNode / ADK 工具循环对齐 |
| `ToolInfo` | `BindTools` / `WithTools` 时描述工具 schema |
| `Document` | **RAG**：索引、检索、切分管线的标准文档单元（`ID`、`Content`、`MetaData`） |
| `StreamReader` / `Pipe` | 流式模型输出与工具回调流式场景 |

---

## 3. `github.com/cloudwego/eino/components/model` — ChatModel 抽象

| 符号（典型） | Claw 中的用途 |
|--------------|----------------|
| `BaseChatModel` | `Generate`、`Stream`：非工具型模型调用（若存在旁路） |
| `ToolCallingChatModel` | **主路径**：支持 `WithTools` / `BindTools`，驱动 ReAct 式工具循环 |
| `Option` | 模型调用级选项（如模型名） |

**实现来源**：具体 `ChatModel` 一般由 **eino-ext** 提供（如 `github.com/cloudwego/eino-ext/components/model/openai`），Claw 仅依赖 **接口** 与配置注入。

---

## 4. `github.com/cloudwego/eino/components/tool` — 工具与 ToolsNode 配置

| 符号（典型） | Claw 中的用途 |
|--------------|----------------|
| `BaseTool` | 注册到 ADK `ToolsConfig`、宿主 `Registry` 的统一工具接口 |
| `utils.InferTool`（等） | 将 Go 函数反射为工具，供内置/示例工具使用 |
| `ToolsNode` 相关类型 | 与 `compose.ToolsNodeConfig` 配合（ADK 内复用） |

Claw 侧另有 **工具 Registry、白名单过滤、MCP 动态注册** —— 装配结果表现为 `[]tool.BaseTool` 或等价集合传入 ADK。

---

## 5. `github.com/cloudwego/eino/components/prompt` — 模板与 ChatTemplate

| 符号（典型） | Claw 中的用途 |
|--------------|----------------|
| `FromMessages` / ChatTemplate 节点 | Compose 图中将变量渲染为 `schema.Message`（可选路径；Claw 亦可仅在 PreTurn 拼接 `[]*schema.Message`） |

用于「声明式 prompt 节点」与 Graph 组合；与 MD 驱动的 PreTurn 二选一或混用。

---

## 6. `github.com/cloudwego/eino/compose` — 编排（Chain / Graph）

| 符号（典型） | Claw 中的用途 |
|--------------|----------------|
| `NewGraph[In, Out]` | 回合外壳：OnReceive → PreTurn → … → PostTurn → OnRespond |
| `AddChatModelNode` | PostTurn 中调用独立「记忆抽取 / Skills 生成」等 **ChatModel** 子节点（若不用完整 ADK Agent） |
| `AddChatTemplateNode` | 上述 prompt 模板节点 |
| `AddLambdaNode` / `InvokableLambda` | PreTurn/PostTurn **确定性逻辑**（读 md、写盘、调用 `memory` 包） |
| `AddEdge` / `START` / `END` | 构图边 |
| `Compile` | 得到可运行实例 |
| **`Runnable` 的 `Invoke` / `Stream`** | 执行整条 compose 流水线；支持与用户可见流对齐 |
| `ToolsNodeConfig` | **ADK `ToolsConfig` 内嵌**，配置工具列表与行为 |
| `WithCallbacks` | 在 **Invoke/Stream** 时注入回调（见 §7） |

Claw **主对话内核**仍以 **ADK `ChatModelAgent`** 为主；Compose 更多承担 **回合级外壳** 与 **异步/串行 PostTurn 节点**。

---

## 7. `github.com/cloudwego/eino/callbacks` — 生命周期回调

| 符号（典型） | Claw 中的用途 |
|--------------|----------------|
| `RunInfo`、`CallbackInput` / `CallbackOutput` | 节点级 OnStart/OnEnd 观测 |
| Handler Builder（如文档中的 `OnStartFn` / `OnEndFn` / `Build`） | **结构化日志、metrics、exec_journal**；与 **业务 workflow 图节点** 解耦 |
| `compose.WithCallbacks(...)` | 编译后执行时传入 |

工具级细粒度回调可使用 **`github.com/cloudwego/eino/utils/callbacks`** 中的 Helper（如 `ToolCallbackHandler`）监控工具参数与返回（可选）。

---

## 8. `github.com/cloudwego/eino/adk` — Agent 开发套件（**核心**）

| 符号（典型） | Claw 中的用途 |
|--------------|----------------|
| `NewChatModelAgent(ctx, *ChatModelAgentConfig)` | 构造 **主对话 / 子 Agent / PostTurn 管线 Agent**（不同 `agent_type` 对应不同 Config） |
| `ChatModelAgentConfig` | **`Name`/`Description`/`Instruction`/`Model`/`ToolsConfig`/`Handlers`** 等 |
| `ToolsConfig` | 内嵌 `compose.ToolsNodeConfig`；**`ReturnDirectly`**、**`EmitInternalEvents`** 等控制工具返回与事件 |
| `ChatModelAgentMiddleware`（`Handlers` 切片） | **回合内**扩展：`WrapModel`、`BeforeModelRewriteState`、`AfterModelRewriteState`、`BeforeAgent`、`WrapInvokableToolCall` / `WrapStreamableToolCall` 等（以当前版本接口为准） |
| `BaseChatModelAgentMiddleware` | 便捷嵌入只实现部分钩子 |
| `ChatModelAgentState` / `ModelContext` | Middleware 中读取或改写状态与模型上下文 |
| **`Agent` 接口与 Runner** | 多 Agent 协作、路由、事件流（进阶场景；参考 eino-examples） |
| `AgentEvent`、事件迭代 | 流式消费 Agent 输出、多步工具事件 |
| **`Interrupt` / `StatefulInterrupt`**（可选） | HITL、人工确认；非一期必达时作为预留 |
| `compose.GetInterruptState`（若与 compose 联用） | 恢复中断状态（官方 HITL 文档场景） |

**Claw 强相关钩子**：

- **`BeforeModelRewriteState`**：多轮 tool 间插入 user 片段、TurnHub insert、动态注入 skills 摘要、预算裁剪前的 message 改写。
- **`WrapModel`**：限流、审计、日志包装。
- **工具包装**：对高风险工具做 `WrapInvokableToolCall` 等（与 harness policy 对齐）。

---

## 9. 可选：RAG 相关组件（`components` + `eino-ext`）

| 区域 | 接口/类型（典型） | Claw 中的用途 |
|------|-------------------|----------------|
| Embedding | `components/embedding` 的 **`Embedder`**、`Embed`、`Option` | 入库与查询向量 |
| Indexer | **`Indexer.Store(ctx, []*schema.Document, ...)`** | 写入向量存储 |
| Retriever | **`Retriever.Retrieve(ctx, query, ...)` → `[]*schema.Document`** | PreTurn 或工具内检索 |
| Document Loader | document loader 组件 | 从磁盘/URL 加载为 `schema.Document` |
| Document Transformer | 如 `eino-ext/.../splitter/markdown` **`Transform`** | 切分长文档再索引 |

具体实现均在 **eino-ext**（Redis、Elasticsearch、OpenAI Embedding 等），Claw 只做 **配置驱动装配**（见 FR-KNOW-*）。

---

## 10. 与 Claw 需求的映射（摘要）

| Claw 能力 | 主要 Eino 落点 |
|-----------|----------------|
| 主会话模型 + 工具循环 | `adk.NewChatModelAgent` + `ToolCallingChatModel` + `ToolsConfig` |
| PreTurn / PostTurn 确定性步骤 | `compose.Graph` / Chain + `AddLambdaNode` + 可选 `AddChatModelNode` |
| 动态消息与多轮插入 | `ChatModelAgentMiddleware.BeforeModelRewriteState`（等） |
| 流式回复 / 取消 | `Runnable.Stream`、`ChatModel.Stream`、ADK 事件流（按官方 API） |
| 观测与审计 | `callbacks` + `compose.WithCallbacks`；工具回调 Helper（可选） |
| 多 Agent / 专用演进 Agent | 多个 `NewChatModelAgent` 实例或 ADK 协作模式；见 eino-examples |
| 知识库（可选） | `Embedder`、`Indexer`、`Retriever`、`schema.Document` + eino-ext 实现 |
| 测试 Mock | 实现 `ToolCallingChatModel` / `Retriever` 等接口的假实现（FR-EINO-04） |

---

## 11. 版本与迁移提示

| 主题 | 说明 |
|------|------|
| ADK 稳定版 | 文档常见要求 **`eino@v0.5.0` 及以上** 使用当前 ADK 形态 |
| Middleware | **v0.8** 起强调 **`ChatModelAgentMiddleware`**；从早期 Decorator 包模型迁移时参考官方 [release notes / migration](https://www.cloudwego.io/docs/eino/release_notes_and_migration/) |
| 长对话 | 官方提供 **Middleware 组合**（patch tool calls、压缩工具输出、摘要历史等）可与 Claw budget 策略并列考虑 |

---

## 12. 修订记录

| 日期 | 说明 |
|------|------|
| 2026-05-02 | 首版：按 PRD / eino-md-chain-architecture 汇总 Eino & eino-ext 接口面；README / eino 文档 / requirements §6 交叉引用 |
