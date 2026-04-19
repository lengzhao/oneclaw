# 代码边界与已落实项（oneclaw）

只读摘要：**以当前实现为准**。未实现的设计草案与可选演进见 [`todo.md`](todo.md)（**#27** 与 §「出站与 context 可选演进（未实现）」），避免与实现真源重复维护。

---

## 1. 已落实（摘要）

| 主题 | 现状 |
|------|------|
| `SubmitUser` / `submitLocalSlashTurn` 共享准备 | `session/turn_prepare.go`：`prepareSharedTurn` |
| 本地 slash 与维护 | 旁路，不跑 `PostTurn` / 维护（[memory-maintain-dual-entry-design.md](memory-maintain-dual-entry-design.md) §2.4） |
| `subagent` 前置去重 | `validateNestedHost` / `validateNestedParent` / `stripMetaForNested`（`subagent/run.go`） |
| `toolctx` | `SessionHost` 嵌入 + `ApplyTurnInboundToToolContext`（`mergeTurnInbound`，见 `toolctx/context.go`） |
| `channel.DrainTextReply` | `statichttp` 等出站聚合 |
| `PostTurnInput` | `SubmitUser` 尾部单次构建，供 `PostTurn` 与 `MaybePostTurnMaintain` 共用 |
| Emitter `context` | `Text` 用回合 ctx；`Done` 用 `context.Background()`（见 `engine.go` / `loop/runner.go` 注释） |
| `NestedMemoryPaths` | 已从 `Context` 移除 |

---

## 2. 当前实现边界（与代码对齐）

- **工具 vs 出站**：`tools/builtin.DefaultRegistry()` 注入主会话工具；助手文本与状态经 **`Engine.publishOutbound`** / **`Engine.updateInboundStatus`**（**`*clawbridge.Bridge`**），与工具注册表**正交**。详见 [inbound-routing-design.md](inbound-routing-design.md) §4、§5。
- **`WorkerPool` 与 `Engine`**：`cmd/oneclaw` 使用 **`session.WorkerPool`**，按 `SessionHandle` 分片，**每任务** `MainEngineFactory` **新建 `Engine`，`SubmitUser` 结束后丢弃**；**`MainEngineFactoryDeps.Bridge`** 注入 **`clawbridge.New`** 的实例（出站唯一来源）。单测 / e2e 对 **`session.NewEngine`** 设置 **`eng.Bridge`**（noop 桥）。详见 [inbound-routing-design.md](inbound-routing-design.md) §8、[config.md](config.md)「会话与多通道」。
- **子 agent 工具表**：**`run_agent`** ≈ catalog ∩ 父表 − 元工具；**`fork_context`** ≈ 父表 − 元工具（深度规则见实现）。详见 [inbound-routing-design.md](inbound-routing-design.md) §9、`subagent/registry.go`。

---

## 3. 包边界（保持现状）

- `memory` 不 import `tools/builtin`；由 `builtin.ScheduledMaintainReadRegistry()` 等做依赖倒置，避免随意打通造成循环。
- `tools/registry.go` 与 `loop` 批处理策略已集中，改动优先级低。

---

*Backlog 与架构对照以 [`todo.md`](todo.md) 为准（含 **#19–#22** 工程简化、**#23–#26** 文档索引、**#27** 可选演进）。*
