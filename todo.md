# oneclaw 待办

本文只保留面向下一阶段的待办。已完成的历史阶段不再展开，架构背景见 [docs/architecture.md](docs/architecture.md)，简单易用路线见 [docs/simplicity-roadmap.md](docs/simplicity-roadmap.md)，workflow 技术债见 [docs/workflow-architecture-review.md](docs/workflow-architecture-review.md)。

---

## P0：收窄默认体验

目标：用户通过 `oneclaw init` + `oneclaw run` 能完成最短可用路径，不需要先理解 workflow、Catalog、ADK 或异步后继。

- [x] **梳理 `init` 默认产物**：**`//go:embed templates`** 嵌入 **`setup/templates/`** 整树，`Bootstrap` 遍历拷到 UserDataRoot（缺才写）；**`config.yaml`** 仍单独合并；**`agents/default.md`** 仍 Go 模板渲染 **`UserDataRoot`**；**`sessions/`**、**`prompts/`**、**`knowledge/sources/`** 空目录保留；**`examples/skills/`** 与模板对齐供浏览；**`tools.exec` 默认保持开启（配置里 deny 基础危险分隔符）**（见 `setup/bootstrap.go`、`setup/embed.go`）。
- [ ] **补充 5 分钟上手文档**：新增面向普通用户的入口文档，覆盖 `init`、配置模型、`run`、常见失败。
- [ ] **统一最短路径验收**：增加或整理烟测命令，覆盖 mock 模型与真实 OpenAI compatible 配置两种路径。

---

## P1：explain 与诊断

目标：让用户少读 YAML / config，也能理解系统当前状态。

- [ ] **`oneclaw config show`**：展示合并后的有效配置，敏感值脱敏，明确配置来源与默认值。
- [ ] **`oneclaw agent list`**：展示内置与用户 Agent、工具白名单、模型覆盖、workspace / memory 策略。
- [ ] **`oneclaw agent new <name>`**：生成最小 `agents/<name>.md`，降低新 Agent 创建成本。
- [ ] **`oneclaw agent run <name> <prompt>`**：提供不改 workflow 的直接 Agent 运行入口。
- [ ] **`oneclaw workflow explain <name>`**：把 `workflows/*.yaml` 翻译成阶段化说明，例如 PreparePrompt → RunMainADK → Respond → PostTurnAsync。
- [ ] **错误提示分层**：配置缺失、模型 endpoint、workflow 校验、工具不存在、workspace 不可写、异步节点失败等错误都包含「发生了什么 / 为什么失败 / 怎么修 / 相关路径」。

---

## P2：降低运行时复杂度

目标：先拆结构，降低 `engine.RuntimeContext` 的心智负担，再考虑更深层重构。

- [ ] **拆分运行时状态结构**：按 `TurnState`、`SessionState`、`WorkflowState`、`PromptState`、`RuntimeServices` 的方向收敛字段职责。
- [ ] **收紧异步与 journal 契约**：明确 sync 完结点、`run_complete` 顺序、async 节点失败记录和可观测性。
- [ ] **为 `RuntimeContext` 拆分补测试**：优先覆盖 node output、async completion、transcript / run journal 写入顺序。
- [ ] **评估 workflow runnable 缓存**：默认图很小时不急做；当 workflow 复杂后再按 workflow id 缓存 Compose 结果。

---

## P3：文档分层

目标：保留当前架构文档深度，同时给普通用户更短入口。

- [ ] **新增 `docs/user-guide.md`**：日常使用、配置、运行、常见错误。
- [ ] **新增 `docs/concepts.md`**：解释 Agent、Session、Memory、Tool、Workflow、Workspace 等核心概念。
- [ ] **新增 `docs/dev-guide.md`**：贡献者如何加工具、加 workflow 节点、加命令入口。
- [ ] **调整 `docs/README.md` 阅读路径**：区分普通用户、进阶用户、实现者 / 架构评审者。

---

## P4：高级能力（默认关闭或后置）

目标：继续保留扩展方向，但不压过默认体验。

- [ ] **RAG**：按 FR-KNOW-* 与 eino-ext 装配 Embedder / Indexer / Retriever；知识库路径默认 `knowledge/sources/`。
- [ ] **浏览器 / Web fetch**：默认关闭或强约束；接入前先定义权限、配额、审计。
- [ ] **MCP**：工具发现、搜索、桥接；与工具白名单和 Harness 策略一起设计。
- [ ] **Harness**：SafeHarness、高风险工具包装、`exec` 配额 / 审计增强。

---

## 维护约定

- 新功能开发前先检查 `docs/` 下相关设计文档，并保持文档和代码一致。
- 待办完成后在本文件勾选，并在相关 PR / 提交说明中标注对应文档或 FR 编号。
- 已完成的大段历史不要重新堆回本文；需要追溯时看 git 历史和设计文档。
