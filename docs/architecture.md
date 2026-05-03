# Claw 运行时架构 — 主流程与生命周期

本文用 **文字 + Mermaid** 梳理 **主路径** 与关键子系统的 **生命周期**，与 [reference-architecture.md](reference-architecture.md)（原则）、[eino-md-chain-architecture.md](eino-md-chain-architecture.md)（Eino 挂载）、[appendix-data-layout.md](appendix-data-layout.md)（目录）、[requirements.md](requirements.md)（FR）一致。Eino **包级清单**见 [eino-integration-surface.md](eino-integration-surface.md)。

---

## 1. 文档定位

| 读者 | 本文提供 |
|------|-----------|
| 产品 / 架构 | 端到端流程、阶段划分、与渠道/数据的关系 |
| 实现 | 与 TurnHub、Compose、ADK、链后继节点、落盘的对齐关系 |
| 评审 | 演进枝叶（workflow）、子 Agent、Workspace 默认策略的可视化 |

---

## 2. 系统上下文（边界）

```mermaid
flowchart TB
  subgraph users [用户与外部]
    U[用户 / 定时器 / Webhook]
  end
  subgraph channels [渠道层]
    B[Bus / clawbridge 可选]
    D[CLI / HTTP / IM drivers]
  end
  subgraph runtime [Claw 运行时]
    TH[TurnHub]
    EN[Engine 每回合新建]
    CF[Compose 回合外壳<br/>含 workflows/*.yaml]
    ADK[Eino ADK 主对话]
    NEXT[主 Agent 之后的链段<br/>记忆/skills 等节点]
  end
  subgraph disk [用户主目录数据]
    UD[(UserDataRoot)]
    SR[(SessionRoot / InstructionRoot)]
    AG[.agent Catalog / workflows]
  end
  subgraph llm [模型与可选 RAG]
    API[Chat API]
    RAG[Embedder / Retriever]
  end
  U --> D
  D --> B
  B --> TH
  TH --> EN
  EN --> CF
  CF --> ADK
  ADK --> NEXT
  ADK --> API
  NEXT --> API
  CF --> SR
  NEXT --> SR
  EN --> AG
  CF --> RAG
```

### 2.1 「PostTurn」与 `workflows/*.yaml`：编排即可，不必单独子系统

**可以**：记忆抽取、Skills 生成等 **全部是 Compose 图上的普通节点**，在 **`workflows/*.yaml`**（DAG；或由 manifest 引用的 workflow）里 **分支 / 汇合 / 异步枝** 声明即可 —— **不需要** 名为 PostTurn 的独立运行时模块；只要 **Workflow 注册表 + 执行器** 能实例化节点并沿边执行。字段、图模型与内置 `use` 见 **[workflows-spec.md](workflows-spec.md)**。

文档里的 **PostTurn** 仅是 **阶段别名**：指「**主对话 ADK 跑完之后**，在同一回合编排里 **排在后面的那一段链**」。实现上就是 **YAML 里 ADK 节点之后的若干节点**。

仍需显式策略的两点（**与是否叫 PostTurn 无关**）：

1. **异步**：用户应先收到 **OnRespond / Bus**，演进类节点 **后台执行** —— 在 YAML / manifest 用 **`async`、分叉边、队列** 等表达，由宿主解释（见 [eino-md-chain-architecture.md](eino-md-chain-architecture.md) §7）。
2. **编排**：主会话在 **`workflows/*.yaml`** 里用 **`memory_agent` / `skill_agent`** 等 **`use: agent` + `async: true`** 枝叶声明记忆抽取与 Skills（见 [workflows-spec.md](workflows-spec.md) §4.3、§8）；默认内置 Catalog 条目可被用户覆盖。**当前 oneclaw** **未**实现演进专用的加载期闭环校验，也 **未**在 **`TurnContext`** 上维护嵌套演进剖面。

---

## 3. 端到端主流程（单轮用户消息）

从入站到回复可见的 **主干**（省略流式 chunk 细节）。

```mermaid
sequenceDiagram
  participant Ch as 渠道
  participant TH as TurnHub
  participant E as Engine
  participant R as Compose Runnable
  participant P as PreTurn
  participant A as ADK 主 Agent
  participant O as OnRespond
  participant Bus as Outbound Bus
  participant PT as 链后继节点<br/>（YAML；常异步）

  Ch->>TH: InboundMessage
  TH->>E:  dequeue / 策略 serial|insert
  E->>R: Invoke TurnContext
  R->>P: OnReceive 已写入上下文
  P->>P: 拼 Instruction / MEMORY / skills 摘要<br/>budget 裁剪
  P->>A: messages + tools + middleware
  loop ReAct 多步
    A->>A: BeforeModelRewriteState 等
    A->>A: ChatModel + Tools
  end
  A->>O: 回合结果文本 / 工具轨迹
  O->>O: transcript / 执行记录落盘
  O->>Bus: publishOutbound
  Bus->>Ch: 用户可见回复
  O->>PT: 触发编排后继<br/>（可与 O 同步或异步，由 workflow 定义）
  Note over PT: workflows/*.yaml<br/>respond 后 async 枝<br/>（如 memory_agent）
```

---

## 4. 回合外壳生命周期（Compose 阶段）

与 [eino-md-chain-architecture.md](eino-md-chain-architecture.md) §3 对齐：**确定性节点** + **内核 ADK**。图中 **Q** 在实现上 **就是 workflow 图中主 ADK 之后的子图（常为 respond 出边或多枝）**，不必单独「PostTurn 服务」。

```mermaid
flowchart LR
  subgraph phases [单回合阶段]
    R[OnReceive<br/>校验 / 附件 / TurnContext]
    P[PreTurn<br/>md → Instruction<br/>MEMORY 快照<br/>Registry filter]
    A[ADK<br/>主 agent_type]
    Q[链后继<br/>YAML 声明]
    O[OnRespond<br/>裁剪 / transcript<br/>runs 记录 / Bus]
  end
  R --> P --> A --> Q --> O
```

**持久化触点**：

- **OnReceive / OnRespond**：会话 transcript、本轮 **`runs/<agent_type>/`** 执行记录（见 FR-AGT-05 / FR-OBS-04）。
- **链后继（原 PostTurn 语义）**：MEMORY / skills 写入（经 staging / policy）；形状由 **YAML** 声明（**`async` 枝叶**）；与实现对齐见 [workflows-spec.md](workflows-spec.md) §6。

---

## 5. ADK 内核生命周期（单 Agent 一次运行）

单轮内 **模型 ↔ 工具** 循环与 Middleware 钩子（概念顺序；以 Eino 实际 API 为准）。

```mermaid
flowchart TD
  START([进入 ADK Run]) --> BA[BeforeAgent<br/>可选：整轮一次]
  BA --> LOOP{还有步数预算?}
  LOOP -->|否| END([结束 / 产出])
  LOOP -->|是| BM[BeforeModelRewriteState<br/>改 messages / 注入片段]
  BM --> WM[WrapModel<br/>可选包装 ChatModel]
  WM --> GEN[Generate 或 Stream]
  GEN --> DEC{需工具调用?}
  DEC -->|否| END
  DEC -->|是| TC[工具执行<br/>WrapInvokableToolCall 可选]
  TC --> AM[AfterModelRewriteState<br/>可选]
  AM --> LOOP
```

**Claw 关注点**：TurnHub insert、动态技能摘要、budget 裁剪 —— 多落在 **`BeforeModelRewriteState`**；观测可走 **callbacks**（见 [eino-integration-surface.md](eino-integration-surface.md)）。

---

## 6. 链后继演进生命周期（记忆 / Skills）

以下逻辑 **完全可用 `workflows/*.yaml` 的 DAG 中若干 `agent` 节点表达**；图仍沿用「主回合之后」的语义。展示 **角色拆分**（**`async`**）；默认模板为线性串联两 async 节点，并行扇出需自行构图（见 [workflows-spec.md](workflows-spec.md)）。

```mermaid
flowchart TD
  MAIN([主对话 on_respond 完成]) --> Q[workflow 出边<br/>workflows/*.yaml]
  Q --> M["memory_agent<br/>use: agent async: true<br/>agent_type: memory_extractor"]
  Q --> S["skill_agent<br/>use: agent async: true<br/>agent_type: skill_generator"]
  M --> DISK1[(MEMORY / staging)]
  S --> DISK2[(skills / staging)]
  M --> LOG1[runs 落盘]
  S --> LOG2[runs 落盘]
  DISK1 --> DONE([结束])
  DISK2 --> DONE
  LOG1 --> DONE
  LOG2 --> DONE
```

**硬性规则**：记忆抽取与 Skills 生成 **不在 Catalog 上用布尔开关声明**；由 **YAML 枝叶** + **内置 / 可覆盖的 Catalog 条目** 表达。**当前** **无**演进专用的加载期闭环校验与 **`TurnContext` 演进剖面**（见 [requirements.md](requirements.md) FR-FLOW-05）。

---

## 7. 子 Agent 生命周期（委托 / `run_agent`）

默认 **会话隔离 + 上下文隔离**；**Workspace 默认与主回合共享**（[requirements.md](requirements.md) FR-AGT-06）。

```mermaid
flowchart LR
  subgraph parent [主会话]
    MA[主 ADK]
    WS[(workspace<br/>shared 默认)]
  end
  subgraph child [子 Agent]
    SA[子 ADK<br/>独立 messages]
    WSP[(workspace<br/>private 时独占目录)]
  end
  MA -->|run_agent / 工具| SA
  MA -.->|shared| WS
  SA -.->|shared| WS
  SA -.->|private| WSP
  SA --> H[handoff：结构化摘要<br/>回主会话 / 工具返回]
  H --> MA
```

**落盘**：子运行优先使用 **`sessions/<parent>/subs/<sub_run>/`** 命名空间写 **runs / 可选 transcript**，避免与父 SessionRoot 混写（[appendix-data-layout.md](appendix-data-layout.md) §3.1）。

---

## 8. 会话与目录生命周期（推荐默认：会话隔离）

```mermaid
flowchart TB
  subgraph global [UserDataRoot]
    CFG[config.yaml]
    MAN[.agent/manifest<br/>agents / workflows / prompts]
  end
  subgraph session [SessionRoot = InstructionRoot]
    AG[AGENT.md]
    MEM[MEMORY / memory/]
    WS[workspace/]
    TR[transcript / runs/]
  end
  SH[SessionHandle 稳定映射] --> SID[session_id]
  SID --> session
  MAN -->|Catalog 只读| RUN[运行时装配]
  session --> RUN
```

**扁平模式**（关闭隔离）：`InstructionRoot = UserDataRoot`，形状类似但根路径不同（见附录 §2）。

---

## 9. 可选：知识库 RAG 生命周期

与主对话 **并行可选**；索引为派生，**原文真源**仍在配置目录（FR-KNOW-*）。

```mermaid
flowchart LR
  SRC[(knowledge/sources/)] --> LOAD[Loader / Splitter]
  LOAD --> EMB[Embedding]
  EMB --> IDX[Indexer.Store]
  IDX --> STORE[(向量后端)]
  Q[用户查询 / PreTurn] --> RET[Retriever]
  STORE --> RET
  RET --> CTX[上下文块 + 来源 id]
  CTX --> ADK[Eino ADK / Prompt]
```

---

## 10. 合成入站（定时）生命周期

与普通消息 **同形、同路径**（仅来源不同）。

```mermaid
flowchart LR
  J[scheduled_jobs.json] --> POL[Poller]
  POL --> SYN[构造 InboundMessage]
  SYN --> TH[TurnHub]
  TH --> SUB[Submit / Engine<br/>与人工消息一致]
```

---

## 11. 相关文档索引

| 主题 | 文档 |
|------|------|
| 术语 | [glossary.md](glossary.md) |
| 原则与场景 PRD 条目 | [reference-architecture.md](reference-architecture.md) |
| Eino + MD + Workflow 细节 | [eino-md-chain-architecture.md](eino-md-chain-architecture.md) |
| `workflows/*.yaml` 规格 | [workflows-spec.md](workflows-spec.md) |
| Eino API 清单 | [eino-integration-surface.md](eino-integration-surface.md) |
| 路径与隔离 | [appendix-data-layout.md](appendix-data-layout.md) |
| 功能 ID / 验收 | [requirements.md](requirements.md) |
| Harness 扩展 | [harness-governance-extensions.md](harness-governance-extensions.md) |

---

## 12. 修订记录

| 日期 | 说明 |
|------|------|
| 2026-05-02 | 首版：系统上下文、端到端序列图、Compose/ADK/子Agent/目录/RAG/定时 生命周期图；§2.1 后继编排；`chains` → `workflows`（DAG）；索引 [workflows-spec.md](workflows-spec.md) |
