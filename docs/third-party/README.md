# 第三方调研（非 oneclaw 设计真源）

本目录存放**外部项目、工具或上游产品**的调研与能力整理，以及 **Claude Code 范式对照长文**（由原 `docs/claude-code-*.md` 迁入），用于对照与借鉴，**不构成** oneclaw 的实现规格或验收标准。

| 与 `docs/` 根目录的区别 | 说明 |
|-------------------------|------|
| **`docs/*.md`（根目录）** | 本仓库 **Agent / Memory 运行时** 的设计真源与阶段任务（与 [`README.md`](../README.md) 索引一致）。 |
| **`docs/third-party/`** | 第三方对象与 **Claude Code 对照**；可能引用 vend 目录（如 `third_repo/…`），内容随上游变更而过时。 |

---

## Claude Code 对照文（归档）

基于历史 TS 参考实现整理的范式说明；与 oneclaw 的**异同**见 [`claude-code-vs-oneclaw.md`](claude-code-vs-oneclaw.md)。

| 文档 | 说明 |
|------|------|
| [`claude-code-vs-oneclaw.md`](claude-code-vs-oneclaw.md) | **oneclaw ↔ Claude Code** 产品/运行时异同（必读桥接） |
| [`claude-code-main-flow-analysis.md`](claude-code-main-flow-analysis.md) | 主流程与分层 |
| [`claude-code-memory-system.md`](claude-code-memory-system.md) | 记忆系统 |
| [`claude-code-subagent-system.md`](claude-code-subagent-system.md) | 子 Agent |
| [`claude-code-skills-mechanism.md`](claude-code-skills-mechanism.md) | Skills 机制 |
| [`claude-code-callstack-and-parameter-flow.md`](claude-code-callstack-and-parameter-flow.md) | 调用栈与参数流 |
| [`claude-code-core-tools.md`](claude-code-core-tools.md) | 核心工具 |
| [`claude-code-agenttool-deep-dive.md`](claude-code-agenttool-deep-dive.md) | Agent 工具深入 |

---

## 官方与其它第三方

| 文档 | 对象 | 说明 |
|------|------|------|
| [`claude-code-hooks-reference.md`](claude-code-hooks-reference.md) | [Claude Code Hooks](https://code.claude.com/docs/en/hooks)（Anthropic） | 官方 **Hooks** 生命周期事件、matcher、决策语义 |
| [`oh-my-claudecode-capabilities.md`](oh-my-claudecode-capabilities.md) | [oh-my-claudecode](https://github.com/Yeachan-Heo/oh-my-claudecode) | OMC 能力、预制 Skill、Team 流程；基于 `third_repo/oh-my-claudecode` |

后续若有更多第三方调研，在本目录新增文件并在上表追加条目即可。
