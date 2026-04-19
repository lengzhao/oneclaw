# Claude Code Hooks 接口调研（官方对照）

> **范围**：本文档整理 **Anthropic Claude Code** 产品的 **Hooks** 机制，供本仓库后续扩展「生命周期 / 观测 / 拦截类」能力时对照；**不是** oneclaw 的当前实现规格。

**权威来源**（以官方为准，产品迭代会变）：[Hooks reference](https://code.claude.com/docs/en/hooks)（含各事件的 JSON 输入输出、决策字段、示例）；快速上手：[Automate workflows with hooks](https://code.claude.com/en/hooks-guide)。

---

## 1. Hooks 是什么

- **定义**：在用户配置的 **生命周期锚点** 上，自动执行 **shell 命令、HTTP、LLM prompt 或 agent**（handler 类型因事件而异，如 `SessionStart` 仅支持 `type: "command"`，以官方文档为准）。
- **输入**：对 **command** 类 hook，Claude Code 将 **JSON 上下文写在 stdin**；HTTP 则为 POST body。
- **输出**：handler 可向 stdout 打印 JSON，用于 **注入上下文、修改工具入参、允许/拒绝/追问** 等；**exit code** 语义因事件而异（见 §5）。

---

## 2. 生命周期事件一览（官方表格）

以下事件名与含义摘自官方 **Hooks reference** 的 **Hook lifecycle** 总表（整理时共 **26** 个事件；若官方增删，以链接页面为准）。


| 事件名                  | 触发时机（摘要）                                                          |
| -------------------- | ----------------------------------------------------------------- |
| `SessionStart`       | 会话开始或恢复                                                           |
| `SessionEnd`         | 会话终止                                                              |
| `UserPromptSubmit`   | 用户提交 prompt，**Claude 处理之前**                                       |
| `Stop`               | Claude **结束本轮回复**时                                                |
| `StopFailure`        | 本轮因 **API 错误**结束；官方说明 **忽略** handler 的输出与 exit code               |
| `PreToolUse`         | **工具调用执行前**；可 **阻止** 或改写                                          |
| `PostToolUse`        | 工具调用 **成功之后**                                                     |
| `PostToolUseFailure` | 工具调用 **失败之后**                                                     |
| `PermissionRequest`  | **权限弹窗**即将出现                                                      |
| `PermissionDenied`   | 工具被 **自动模式分类器拒绝**；可返回 `{retry: true}` 让模型重试该次调用                   |
| `Notification`       | Claude Code 发出 **通知**                                             |
| `SubagentStart`      | **子代理**被创建                                                        |
| `SubagentStop`       | **子代理**结束                                                         |
| `TaskCreated`        | 通过 `TaskCreate` **创建任务**时                                         |
| `TaskCompleted`      | 任务被标为 **完成**时                                                     |
| `TeammateIdle`       | [Agent team](https://code.claude.com/en/agent-teams) 中队友 **即将空闲** |
| `PreCompact`         | **上下文压缩**之前                                                       |
| `PostCompact`        | **上下文压缩**完成之后                                                     |
| `InstructionsLoaded` | `CLAUDE.md` 或 `.claude/rules/*.md` **加载进上下文**时（会话开始或懒加载等）         |
| `ConfigChange`       | **会话中**配置文件变更                                                     |
| `CwdChanged`         | **工作目录**变化（如执行 `cd`）                                              |
| `FileChanged`        | **被监视的文件**在磁盘上变化；`matcher` 用于文件名规则                                |
| `WorktreeCreate`     | 通过 `--worktree` 或 `isolation: "worktree"` **创建工作树**               |
| `WorktreeRemove`     | **移除工作树**（退出会话或子代理结束等）                                            |
| `Elicitation`        | **MCP 服务器**在工具调用中请求用户输入                                           |
| `ElicitationResult`  | 用户对 MCP elicitation **作答后**、结果回传服务器 **之前**                        |


**节奏分类（官方叙述）**：  

- **每会话**：如 `SessionStart`、`SessionEnd`  
- **每轮（turn）**：如 `UserPromptSubmit`、`Stop`、`StopFailure`  
- **agent 循环内每次工具**：`PreToolUse`、`PostToolUse`、`PostToolUseFailure` 等

---

## 3. Matcher 与部分事件的过滤字段

- **matcher**：按事件类型过滤 **不同字段**（例如工具类事件多按 **tool name**；`SessionStart` 按会话启动方式 `startup` / `resume` / `clear` / `compact` 等）。
- **无 matcher 支持**（官方文档说明 **始终每次触发**，若写 `matcher` 可能被**静默忽略**）：`UserPromptSubmit`、`Stop`、`TeammateIdle`、`TaskCreated`、`TaskCompleted`、`WorktreeCreate`、`WorktreeRemove`、`CwdChanged` 等。
- `**if` 条件**：可与 permission 规则语法类似，进一步缩小 **工具类** hook 的执行范围（如仅 `Bash(git *)`），详见官方 **Configuration**。

子代理场景：官方说明对子代理会将部分 `Stop` hook 行为映射为 `**SubagentStop`**（见官方文档 *Async hooks* / subagent 相关章节）。

---

## 4. 配置位置（作用域）


| 位置                              | 作用域                 |
| ------------------------------- | ------------------- |
| `~/.claude/settings.json`       | 本机所有项目              |
| `.claude/settings.json`         | 单项目（可提交仓库）          |
| `.claude/settings.local.json`   | 单项目本地（通常 gitignore） |
| 托管策略                            | 组织策略                |
| **Plugin** 的 `hooks/hooks.json` | 插件启用时               |
| **Skill / Agent** frontmatter   | 组件激活期间              |


企业策略可使用 `allowManagedHooksOnly` 等限制用户/项目/插件 hook（见官方 settings 文档）。

---

## 5. 决策能力与 exit code（摘要）

官方文档对 **exit code** 与 **JSON 决策** 有完整矩阵，此处仅列常见点：


| 行为                                               | 说明                                                                                                                    |
| ------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------- |
| **exit 0**                                       | 成功；stdout 中 JSON 若合法则按事件解析                                                                                            |
| **exit 2**                                       | 阻塞类错误；**stderr** 反馈给 Claude；对 `PreToolUse` 等会 **阻止** 工具执行                                                             |
| `**PreToolUse*`*                                 | 使用 `hookSpecificOutput`（如 `permissionDecision`: allow/deny/ask/defer），多 hook 决策优先级：`deny` > `defer` > `ask` > `allow` |
| `**UserPromptSubmit` / `Stop` / `PreCompact` 等** | 部分支持顶层 `decision: "block"` 等（见官方各事件小节）                                                                                |
| `**StopFailure`**                                | **无**决策控制                                                                                                             |


**UserPromptSubmit / SessionStart**：stdout 内容在部分场景会**进入模型可见上下文**（官方说明例外于「仅写 debug log」的通用规则）。

---

## 6. Handler 类型（扩展）

官方文档除 **command** 外，还支持 **HTTP、prompt、agent** 等形态（见 Hooks reference 全文与 hooks-guide），并可能有 **async** 等高级特性；实现细节以官方为准。

---

## 7. 与 oneclaw 后续「Hook 能力」的对照思路

oneclaw 当前与「观测 / 生命周期」相关的设计见 [**notification-hooks-design.md**](../notification-hooks-design.md)（进程内 `notify.Sink`、事件类型、correlation、`agent_id` 等），**并非** Claude Code 的 `hooks.json` 协议。


| Claude Code 概念             | oneclaw 可对齐的方向                                                                                  |
| -------------------------- | ----------------------------------------------------------------------------------------------- |
| **会话/轮次/工具** 分层事件          | 在 `SubmitUser`、`loop.RunTurn`、工具前后、`PostTurn`、子 agent 等路径上 **统一枚举事件名**，与现有 notify 事件对齐或做别名表     |
| **PreToolUse 拦截**          | 已有 `CanUseTool` / 工具白名单；若需「外部策略」可扩展为 **同步策略接口** 或 **异步审计**（注意延迟）                                |
| **UserPromptSubmit 注入上下文** | 类似「回合前注入」：与 `turn_prepare`、`memory` 注入顺序对齐，避免重复膨胀                                               |
| **Stop 续跑**                | 与维护/任务完成语义不同；若做「禁止早停」需单独产品语义，避免与 `Abort` 冲突                                                     |
| **stdin JSON 协议**          | oneclaw 为 Go 进程，更适合 **Go interface + 可选 exec 插件**，而不是照搬 Node stdin 格式；可定义 **稳定 JSON schema 版本** |


**建议**：将 Claude Code 的 **26 个事件** 当作「能力清单」；oneclaw 实现时 **按优先级** 子集落地（例如先对齐：入站、turn 开始/结束、模型步、工具前后、子 agent 起止），并固定 **schema_version** 便于演进。

---

## 8. 参考链接

- [Hooks reference](https://code.claude.com/docs/en/hooks)（事件表、JSON、exit code、matcher）
- [hooks-guide](https://code.claude.com/en/hooks-guide)
- 本仓库内 OMC 使用的子集（11 个事件）见 [oh-my-claudecode-capabilities.md](oh-my-claudecode-capabilities.md) 与 `third_repo/oh-my-claudecode/docs/HOOKS.md`（**仅为插件示例，不全等于官方全集**）

