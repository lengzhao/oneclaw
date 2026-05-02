# Claw / Agent 运行时 — 设计文档套件（本仓库为 `docs/`）

本目录是一套**可整体复制**的文字规格：产品简介、抽象架构、目标技术栈（Eino + MD + **Workflow/Graph**）、**目标 PRD**、术语与数据布局附录。**套件内部交叉引用仅使用本目录内相对路径**；复制到其他仓库时可改名为 `docs/design/` 等任意路径，保持相对链接即可。

---

## 项目简介

**Claw** 这一类产品的目标：**用 Agent 运行时连接模型、工具与渠道，把用户意图落成可重复的自动化**——读写工作区、执行命令、调度提醒、多轮推理与子 Agent 委派，在对话或集成界面**交付结果**，而不止于单次问答。

常驻多通道（IM 等）侧推荐 Go 模块 **`github.com/lengzhao/clawbridge`** 与运行时对接。

---

## 文档一览与依赖关系

| 文件 | 角色 | 复制到新项目时 |
|------|------|----------------|
| [README.md](README.md) | 本索引与复制指南 | **必留** |
| [glossary.md](glossary.md) | 术语统一 | **建议保留**（可与 README 合并） |
| [appendix-data-layout.md](appendix-data-layout.md) | UserDataRoot / InstructionRoot / 隔离策略摘要 | **建议保留**（落地路径设计时对照） |
| [reference-architecture.md](reference-architecture.md) | 架构原则 + 场景化 PRD 条目 + 落地顺序 | **建议保留** |
| [architecture.md](architecture.md) | **主流程 + 各子系统生命周期**（Mermaid） | **建议保留** |
| [eino-md-chain-architecture.md](eino-md-chain-architecture.md) | Eino + 全 MD + `agents/` + **Workflow（Graph）** | **选 Go+Eino 时核心** |
| [workflows-spec.md](workflows-spec.md) | **`workflows/*.yaml` Graph、`steps` 糖、manifest** | **实现编排必读** |
| [eino-integration-surface.md](eino-integration-surface.md) | **Eino / eino-ext 接口与包清单**（实现对照） | **实现工程师必读** |
| [harness-governance-extensions.md](harness-governance-extensions.md) | Harness 治理、SafeHarness 映射、**扩展 backlog** 与初期预留扩展性 | **增强方向**；一期验收以 requirements 为准 |
| [requirements.md](requirements.md) | **目标产品 PRD**（FR/NFR、验收要点） | **绿场核心**；若产品范围不同可删或替换 |

```mermaid
flowchart TB
  README[README.md]
  G[glossary.md]
  A[architecture.md]
  L[appendix-data-layout.md]
  R[reference-architecture.md]
  E[eino-md-chain-architecture.md]
  H[harness-governance-extensions.md]
  Q[requirements.md]
  README --> G
  README --> A
  README --> L
  README --> R
  README --> E
  README --> H
  README --> Q
  R --> A
  A -.->|流程对齐| E
  R -.->|原则对齐| E
  Q -.->|FR 细化| R
  Q --> L
  R --> G
  E --> G
  H -.->|扩展挂钩| E
  H -.->|非一期必达| Q
```

---

## 推荐阅读顺序

1. **[glossary.md](glossary.md)**（首次阅读扫一遍术语）
2. **[architecture.md](architecture.md)** — **主流程与各生命周期图**（建议第二读）
3. **[reference-architecture.md](reference-architecture.md)** — 边界、架构块、PRD、落地顺序  
4. **[eino-md-chain-architecture.md](eino-md-chain-architecture.md)** — 若技术栈含 Go + Eino  
5. **[workflows-spec.md](workflows-spec.md)** — `workflows/*.yaml`（DAG）与内置节点  
6. **[eino-integration-surface.md](eino-integration-surface.md)** — Eino / eino-ext **包与接口清单**（实现对照）  
7. **[appendix-data-layout.md](appendix-data-layout.md)** — 定目录与隔离策略时（含 **§6 其余推荐默认**）  
8. **[requirements.md](requirements.md)** — PRD 与验收要点  
9. **[harness-governance-extensions.md](harness-governance-extensions.md)** — 治理增强、扩展路线与初期预留扩展性（可选）  

---

## 复制到其他新项目时的说明

1. **整目录拷贝**：将本目录原样放入目标仓库的 `docs/design/`（或任意路径），保持相对链接有效即可。
2. **收窄范围**：若不需要完整 PRD，可删除或改写 **[requirements.md](requirements.md)**，并同步调整其他文档中的交叉引用。
3. **非 Go / 不用 Eino**：保留 [reference-architecture.md](reference-architecture.md) + [glossary.md](glossary.md) + [appendix-data-layout.md](appendix-data-layout.md)；[eino-md-chain-architecture.md](eino-md-chain-architecture.md) 可作「若将来迁移到 Eino」存档或删除。
4. **自洽性**：本套件对外部仓库链接主要包括 **`github.com/lengzhao/clawbridge`**（术语表与架构参考）及 **requirements.md §6** 等；其余多为本目录内 `.md`。
