# E2E 用例清单（Stub 模型）

所有用例默认使用 **`test/openaistub`**：`openai.NewClient(stubOpenAIOptions(stub)...)` 指向 stub URL；**`baseStubTransport`** 将 **`rtopts`** 的 **`chat.transport`** 设为 **`non_stream`**（避免 `auto` 双次请求耗尽 stub 队列）。  
隔离建议：每个用例 **`t.TempDir()` 作 cwd**；需要用户级配置时用 **`t.Setenv("HOME", tmpHome)`** 再写 `~/.oneclaw`。

**状态**：`[ ]` 未实现 · `[x]` 已实现（对应 `*_test.go` 内测试函数名）

---

## 实现总表

| ID | 摘要 | 状态 | 测试函数 / 文件 |
|----|------|------|------------------|
| E2E-01 | 最小对话（纯文本结束） | [x] | `TestE2E_StubTextReply` · `stub_loop_test.go` |
| E2E-02 | 同 session 多轮（两次 `RunTurn` 共享 `Messages`） | [x] | `TestE2E_02_MultiTurnSameSession` · `stub_session_test.go` |
| E2E-03 | 工具调用闭环（read） | [x] | `TestE2E_StubToolThenText` · `stub_loop_test.go` |
| E2E-04 | 写后读（write → read） | [x] | `TestE2E_04_WriteThenRead` · `stub_session_test.go` |
| E2E-05 | Abort（ctx cancel） | [x] | `TestE2E_05_AbortCanceledContext` · `stub_session_test.go` |
| E2E-10 | 用户级 `~/.oneclaw/AGENT.md` 注入 | [x] | `TestE2E_10_UserAgentMdInjected` · `stub_memory_engine_test.go` |
| E2E-11 | 项目 `.oneclaw/AGENT.md`（不再读仓库根 `AGENT.md`） | [x] | `TestE2E_11_ProjectOneclawAgentMd` · `stub_memory_engine_test.go` |
| E2E-12 | 仅 `.oneclaw/AGENT.md` | [x] | `TestE2E_12_DotOneclawAgentMdOnly` · `stub_memory_engine_test.go` |
| E2E-13 | `.oneclaw/rules/*.md` | [x] | `TestE2E_13_DotOneclawRules` · `stub_memory_engine_test.go` |
| E2E-14 | 目录向上遍历优先级 | [x] | `TestE2E_14_WalkUpOrderChildAfterParent` · `stub_memory_engine_test.go` |
| E2E-15 | `rtopts`/`features.disable_memory` 关闭注入 | [x] | `TestE2E_15_MemoryDisabledNoAgentInject` · `stub_memory_engine_test.go` |
| E2E-16 | `HOME` 不可用时的降级 | [x] | `TestE2E_16_NoHomeDegradesGracefully` · `stub_memory_engine_test.go` |
| E2E-20 | 规则 `MEMORY.md` 出现在 `BuildTurn` 的 `AgentMdBlock`（与 AGENT 同级注入） | [x] | `TestE2E_20_MemoryMDInAgentMdBlock` · `stub_memory_bundle_test.go` |
| E2E-21 | 规则 `MEMORY.md` 行数截断 + WARNING（`AgentMdBlock`） | [x] | `TestE2E_21_MemoryMDLineTruncationWarning` · `stub_memory_bundle_test.go` |
| E2E-22 | 规则 `MEMORY.md` 字节截断 + WARNING（`AgentMdBlock`） | [x] | `TestE2E_22_MemoryMDByteTruncationWarning` · `stub_memory_bundle_test.go` |
| E2E-30 | recall 命中关键词 | [x] | `TestE2E_30_RecallHit` · `stub_memory_engine_test.go` |
| E2E-31 | recall 未命中无附件 | [x] | `TestE2E_31_RecallMissNoAttachment` · `stub_memory_engine_test.go` |
| E2E-32 | recall 同路径会话内去重 | [x] | `TestE2E_32_RecallPathDedupSecondTurn` · `stub_recall_more_test.go` |
| E2E-33 | recall 总字节预算 | [x] | `TestE2E_33_RecallTotalByteBudget` · `stub_recall_more_test.go` |
| E2E-40 | `write_file` 仅 cwd 内 | [x] | `TestE2E_40_WriteFileUnderCwdOnly` · `stub_paths_test.go` |
| E2E-41 | `write_file` 到 memory 根（`.oneclaw` / `HOME`） | [x] | `TestE2E_41_WriteFileUnderUserMemoryRoot` · `stub_paths_test.go` |
| E2E-42 | 越权路径拒绝 | [x] | `TestE2E_42_WriteFileRejectedOutsideRoots` · `stub_paths_test.go` |
| E2E-43 | `grep` 在 memory 根内 | [x] | `TestE2E_43_GrepUnderProjectMemoryRoot` · `stub_paths_test.go` |
| E2E-50 | 默认 daily log 追加 | [x] | `TestE2E_50_DailyLogAppendDefault` · `stub_postturn_test.go` |
| E2E-51 | `features.disable_memory_extract`（经 `rtopts`）不写 log | [x] | `TestE2E_51_DailyLogDisabledByEnv` · `stub_postturn_test.go` |
| E2E-52 | `features.disable_auto_memory` 关闭 auto | [x] | `TestE2E_52_AutoMemoryDisabledOmitsAutoBullet` · `stub_memory_bundle_test.go` |
| E2E-60 | transcript 保存再加载 | [x] | `TestE2E_60_TranscriptRoundTrip` · `stub_transcript_test.go` |
| E2E-61 | transcript 损坏 JSON 报错 | [x] | `TestE2E_61_TranscriptInvalidJSON` · `stub_transcript_test.go` |
| E2E-70 | CLI / Sink 事件（可选） | [x] | `TestE2E_70_SinkRegistryTextAndDone` · `stub_sink_test.go` |
| E2E-80 | 无 `openai.api_key`（真客户端路径，可选） | [ ] | — |
| E2E-81 | 空用户输入被拒绝 | [x] | `TestE2E_81_EmptyInboundRejected` · `stub_negative_test.go` |
| E2E-82 | 未知工具名 tool_result | [x] | `TestE2E_82_UnknownToolName` · `stub_negative_test.go` |
| E2E-90 | `run_agent` 子循环 + sidechain 落盘 | [x] | `TestE2E_StubRunAgentNested` · `stub_subagent_test.go` |
| E2E-91 | `fork_context` 共享 system | [x] | `TestE2E_StubForkContext` · `stub_subagent_test.go` |
| E2E-92 | 近场维护写入 `memory/YYYY-MM-DD.md`（测试中显式放开 `DisableAutoMaintenance`） | [x] | `TestE2E_92_AutoMaintenanceAppends` · `stub_maintain_test.go` |
| E2E-93 | PostTurn daily log 落在 `<memory_base>/projects/...`，**不**写入 `memory-write.jsonl` | [x] | `TestE2E_93_PostTurnDailyLogSkipsProjectsAudit` · `stub_audit_test.go` |
| E2E-94 | `features.disable_memory_audit` 不写审计文件 | [x] | `TestE2E_94_MemoryAuditDisabledNoFile` · `stub_audit_test.go` |
| E2E-95 | `write_file` 写 project memory 根时审计含 `write_file` | [x] | `TestE2E_95_MemoryAuditWriteFileUnderMemoryRoot` · `stub_audit_test.go` |
| E2E-96 | `oneclaw -maintain-once` 子进程 + stub 写回当日 `memory/YYYY-MM-DD.md` | [x] | `TestE2E_96_MaintainCLIOnce` · `stub_maintain_cli_test.go` |
| E2E-97 | `oneclaw -init` 子进程写入项目 `config.yaml` | [x] | `TestE2E_97_OneclawInitWritesProjectConfig` · `stub_maintain_cli_test.go` |
| E2E-101 | 近场维护：第 2 次请求仅含 Current turn 快照 + 规则 `MEMORY.md` 摘录；不含 daily log / topic；写回当日 episodic 日文件 | [x] | `TestE2E_101_PostTurnMaintainPromptSessionOnly` · `stub_maintain_pipeline_e2e_test.go` |
| E2E-102 | 维护强去重：规则 `MEMORY.md` 已有同义 bullet 时 episodic 不落 `## Auto-maintained` | [x] | `TestE2E_102_MaintainDedupeSkipsAppendWhenNoNewBullets` · `stub_maintain_pipeline_e2e_test.go` |
| E2E-103 | 语义 compact：预算裁剪时首条 chat 请求 user 文本含 `compact_boundary` | [x] | `TestE2E_103_SemanticCompactInChatRequest` · `stub_semantic_compact_e2e_test.go` |
| E2E-104 | `DisableSemanticCompact`（`rtopts`）时首请求 user 侧无 `compact_boundary` | [x] | `TestE2E_104_SemanticCompactDisabledNoBoundaryTag` · `stub_semantic_compact_e2e_test.go` |
| E2E-105 | Skills：首轮请求 system 含 `## Skills` 与技能名/描述 | [x] | `TestE2E_105_SkillsIndexInSystemPrompt` · `stub_skills_e2e_test.go` |
| E2E-106 | `invoke_skill` 返回正文 + `skills-recent.json` 记录 | [x] | `TestE2E_106_InvokeSkillToolAndRecentFile` · `stub_skills_e2e_test.go` |
| E2E-107 | `DisableSkills`（`rtopts`）时 system 无 Skills 段 | [x] | `TestE2E_107_SkillsDisabledNoSystemSection` · `stub_skills_e2e_test.go` |
| E2E-108 | 存在 `tasks.json` 时 system 含 Task list 摘要 | [x] | `TestE2E_108_TasksBlockInSystemPrompt` · `stub_tasks_e2e_test.go` |
| E2E-109 | `task_create` / `task_update` 落盘；`DisableTasks`（`rtopts`）关闭 system 任务段 | [x] | `TestE2E_109_TaskToolsWriteFileAndDisableHidesBlock` · `stub_tasks_e2e_test.go` |
| E2E-111 | `cron` add 写入 `scheduled_jobs.json` | [x] | `TestE2E_111_CronToolWritesFile` · `stub_schedule_e2e_test.go` |
| E2E-112 | 启用中的定时任务出现在 system「Scheduled jobs」段 | [x] | `TestE2E_112_ScheduledJobsBlockInSystemPrompt` · `stub_schedule_e2e_test.go` |
| E2E-113 | `/help` 本地斜杠不调用模型 | [x] | `TestE2E_113_SlashHelpSkipsModel` · `stub_inbound_orchestration_e2e_test.go` |
| E2E-114 | `Inbound` meta + 附件进入 user 历史 | [x] | `TestE2E_114_InboundMetaAndAttachmentInHistory` · `stub_inbound_orchestration_e2e_test.go` |
| E2E-115 | 空正文 + 附件合法 | [x] | `TestE2E_115_EmptyTextWithAttachmentAccepted` · `stub_inbound_orchestration_e2e_test.go` |
| E2E-116 | Delegated agents：system 含目录段；`run_agent` 工具为静态 description（目录不进工具定义） | [x] | `TestE2E_116_AgentCatalogInSystemAndRunAgentTool` · `stub_agents_catalog_e2e_test.go` |
| E2E-113 | 远场 `RunScheduledMaintain`：stub 首次请求 user 为工具型说明（绝对路径），不内嵌 log/topic 全文；`opts.ToolRegistry` 为只读 builtin | [x] | `TestE2E_113_ScheduledMaintainPromptToolOrientedPaths` · `stub_maintain_pipeline_e2e_test.go` |

---

## 1. 会话与模型闭环

### E2E-01 最小对话

- **前置**：stub `Enqueue(CompletionStop(...))`；`Registry` 可无工具。
- **步骤**：`loop.RunTurn` 一次，`Inbound.Text` 任意非空。
- **期望**：无 error；`Messages` 末条为 assistant，内容与 stub 一致。
- **实现**：`TestE2E_StubTextReply`。

### E2E-02 同 session 多轮

- **前置**：stub 按顺序 `Enqueue` 两段 `CompletionStop`。
- **步骤**：同一 `Messages` 切片上连续两次 `RunTurn`，不同用户句。
- **期望**：两次都有 assistant；history 条数递增；第二轮请求里包含前序内容（可通过 stub 记录收到的 messages 长度或本地断言条数）。
- **实现**：`TestE2E_02_MultiTurnSameSession`（第二轮 `history_messages=4` 覆盖「含前序」）。

### E2E-03 工具调用闭环

- **前置**：`Enqueue(ToolCalls read_file)` + `Enqueue(CompletionStop)`；registry 含 `read_file`；cwd 下有待读文件。
- **期望**：存在 tool 消息；最终 assistant 为第二段 stub 文本。
- **实现**：`TestE2E_StubToolThenText`。

### E2E-04 写后读

- **前置**：stub：`ToolCalls write_file` → `ToolCalls read_file` → `CompletionStop`（或合并为两轮 model，按你编排）。
- **步骤**：参数写入 `subdir/x.txt` 再读同一路径。
- **期望**：tool 结果成功；文件在磁盘存在且内容一致。
- **实现**：`TestE2E_04_WriteThenRead`。

### E2E-05 Abort

- **前置**：stub 第一请求阻塞（可用 `time.Sleep` + 可取消的 context），或慢速响应。
- **步骤**：`RunTurn` 使用已 cancel 的 ctx。
- **期望**：返回 `context.Canceled`；无 panic。
- **实现**：`TestE2E_05_AbortCanceledContext`（**进入模型前**即 cancel，不命中 stub 队列）。

---

## 2. Memory 注入（AGENT.md / `.oneclaw`）

> 使用 **`session.Engine.SubmitUser`** 或自行组装与 `SubmitUser` 等价的 `memory.BuildTurn` + `loop.Config`（与生产一致）。  
> 环境：**不要**在 `rtopts` 中开启 `DisableMemory`（测注入时）；设 **`HOME`** 为 `t.TempDir()`。

### E2E-10 用户级 AGENT.md

- **前置**：`$HOME/.oneclaw/AGENT.md` 含唯一标记字符串 `E2E_MARKER_USER`。
- **步骤**：`SubmitUser` 一轮；stub 返回 `CompletionStop`，内容可带该标记或仅断言请求侧。
- **期望**：注入的 user/meta 消息或 system 后缀中出现该标记（对 `Messages` 或 `memory.BuildTurn` 返回值断言）。

### E2E-11 项目 `.oneclaw/AGENT.md`

- **前置**：`cwd/.oneclaw/AGENT.md` 含项目标记（根目录 `AGENT.md` 不参与注入）。
- **期望**：注入中出现该标记。

### E2E-12 仅 `.oneclaw/AGENT.md`

- **前置**：仅有 `cwd/.oneclaw/AGENT.md`，根目录无 `AGENT.md`。
- **期望**：标记出现。

### E2E-13 `.oneclaw/rules`

- **前置**：`cwd/.oneclaw/rules/x.md` 含 `E2E_MARKER_RULE`。
- **期望**：标记出现。

### E2E-14 向上遍历优先级

- **前置**：父目录与子目录各放不同 `.oneclaw/AGENT.md` 标记；**cwd 为子目录**。
- **期望**：**更靠近 cwd** 的规则在拼接结果中占优（顺序或覆盖语义与 `memory/discover.go` 一致）。

### E2E-15 关闭 memory

- **前置**：`cwd/.oneclaw/AGENT.md` 含不应出现的标记；`e2eEnvMinimal` + `rtopts` 置 `DisableMemory=true`（见实现）。
- **期望**：注入中**无**项目标记。

### E2E-16 HOME 不可用

- **前置**：`t.Setenv("HOME", "/nonexistent_path_e2e")` 或使用无写权限路径（按平台谨慎）。
- **期望**：不崩溃；`SubmitUser` 返回错误或跳过 user 侧 memory（与 `session/engine.go` 行为一致）。

---

## 3. 规则 MEMORY.md 注入与截断

### E2E-20 / E2E-21 / E2E-22

- **前置**：在某一 memory 根下放置规则 `MEMORY.md`（短文本 / >200 行 / 少行大字节）。
- **步骤**：开启 memory，`BuildTurn` 或完整 `SubmitUser`。
- **期望**：`AgentMdBlock`（与 AGENT 同级）中出现规则摘录；截断场景含 **WARNING** 子串（与 `memory/truncate.go` 一致）。

---

## 4. Recall

### E2E-30～E2E-33

- **前置**：memory 根下 `.md`（recall **不**索引根上 `MEMORY.md`；其它根级 `.md` 与日 digest 均可命中）含与用户问题重叠的词；或故意无重叠；或多样文件测总预算。
- **步骤**：`BuildTurn`/`SubmitUser` 带特定 `userText`；同一 `RecallState` 多轮测 E2E-32。
- **期望**：`Attachment: relevant_memories` 出现与否；路径去重；总长度上限。

---

## 5. 工具路径与 memory 根

### E2E-40～E2E-43

- **前置**：`ToolContext` 带与 `session` 一致的 `MemoryWriteRoots`（可用 `memory.DefaultLayout` + `WriteRoots()`）。
- **步骤**：`RunTurn` + stub 下发 `write_file` / `grep`。
- **期望**：cwd 内成功；memory 根内成功；越权失败；grep 在 memory 路径可匹配。

---

## 6. Auto memory 与 daily log

### E2E-50～E2E-52

- **前置**：layout 可写；`PostTurn` 触发条件（默认开启 extract 时）。
- **期望**：`logs/YYYY/MM/YYYY-MM-DD.md` 追加一行；`features.disable_memory_extract` 时不追加；`features.disable_auto_memory` 时 auto 目录行为与文档一致。

---

## 7. Transcript

### E2E-60 / E2E-61

- **前置**：`loop.MarshalMessages` / `Engine.MarshalTranscript` 写入文件；再 `LoadTranscript`。
- **期望**：轮次一致；损坏文件返回 error。

---

## 8. 路由与 CLI（可选）

### E2E-70

- **说明**：若需测 `routing.Emitter`，在 `loop.Config` 挂 sink + `SubmitUser`；或单独子测试调用 CLI（较重）。

---

## 9. 负面与韧性

### E2E-80～E2E-82

- **E2E-80**：不设 key 且 **不**走 stub（可选，非默认 CI）。
- **E2E-81**：`Engine.SubmitUser` 空文本。
- **E2E-82**：stub 返回未注册工具名；期望 tool 错误消息 + 会话可继续或结束（与 `loop` 一致）。

---

## 10. 子 Agent（阶段 C）

### E2E-90 / E2E-91

- **E2E-90**：stub：`ToolCalls(run_agent)` → 子循环 `CompletionStop` → 主循环 `CompletionStop`；期望主 transcript 末条为父级最终回复；`.oneclaw/sidechain/` 有落盘。
- **E2E-91**：stub：`ToolCalls(fork_context)` → 子 `CompletionStop` → 主 `CompletionStop`。

### E2E-92

- **前置**：`HOME` 与 cwd；memory 开启；`DisableAutoMaintenance=false`（`e2eEnvWithMemory` 默认会关掉维护以免多耗 stub）；stub 连续两次 `CompletionStop`（主回合 + 维护回合）。
- **期望**：`<cwd>/.oneclaw/memory/YYYY-MM-DD.md` 含 `## Auto-maintained` 维护段。

### E2E-101～104、113（维护近/远场 + 语义 compact）

- **E2E-101**：预写昨日 daily log 与 topic（用于干扰）；`SubmitUser` 后解析**第 2 次**维护请求：须含 `Current turn snapshot`、本轮 user/assistant 标记；**不得**含 `### Daily log`、昨日标记、topic 标记；当日 **`memory/YYYY-MM-DD.md`** 含新 bullet。
- **E2E-113**（`stub_maintain_pipeline_e2e_test.go`）：`RunScheduledMaintain(..., &ScheduledMaintainOpts{ToolRegistry: builtin.ScheduledMaintainReadRegistry()})`（**按天 LOG_DAYS**，非增量）；预写昨日+今日 log 与 topic（供体量探测）；stub **首次**请求 user 须含 far-field / `read_file` / `write_behavior_policy` 与 auto 根、今日 log、**规则** `MEMORY.md`、**当日 digest 路径**、project memory 目录的**绝对路径**，且**不得**内嵌 log/topic 标记正文；写回当日 **`memory/YYYY-MM-DD.md`**。
- **E2E-102**：预写规则 `MEMORY.md` 含与维护模型输出**同义**的 bullet；维护后 `MEMORY.md` 与种子一致，且**不**在当日 episodic 文件追加 `## Auto-maintained`（强去重 `no_new_facts_after_dedupe`）。
- **E2E-103**：`e2eEnvMinimal` + `loop.RunTurn`；预置大量 user 消息 + 较小 `rtopts` 预算 `MaxPromptBytes`；**首次**请求的 user 拼接文本含 `compact_boundary`。
- **E2E-104**：同 E2E-103 体量，但 `DisableSemanticCompact=true`；首次请求 user 文本**不含** `compact_boundary`（纯丢头裁剪）。

---

## 11. Memory 审计（阶段 D2）

### E2E-93～E2E-95

- **E2E-93**：默认开启审计（对非 projects 路径仍生效）；`SubmitUser` 后 `PostTurn` 写当日 daily log；断言 log 文件存在，且审计文件中**无** `source=daily_log_line`（`<memory_base>/projects/` 不落审计）。
- **E2E-94**：`features.disable_memory_audit` 时上述审计文件不存在。
- **E2E-95**：stub 下发 `write_file` 至 `<cwd>/.oneclaw/memory/...`；审计中至少一行 `source=write_file` 且 `path` 为绝对目标路径。

## 12. 维护与初始化 CLI（`cmd/oneclaw`）

### E2E-96

- **前置**：预写当日 daily log（定时路径读 log）；受 YAML **`maintain.min_log_bytes`** 等约束；stub 一次 `CompletionStop`（维护段 + 唯一标记）；子进程配置含 **`openai.base_url`**、**`chat.transport: non_stream`**；子进程 `HOME` 为 `t.TempDir()` 以隔离 memory layout。
- **E2E-96**：`go build ./cmd/oneclaw` 后执行 `-cwd <tmp> -maintain-once`，期望当日 **`memory/YYYY-MM-DD.md`** 含维护标记。
- **说明**：`go build` 子进程将 `HOME` 设为包 init 时保存的真实 HOME，避免模块缓存写入 `t.TempDir()` 导致只读文件清理失败。

### E2E-97

- **E2E-97**：`go build ./cmd/oneclaw` 后执行 `-cwd <tmp> -init`，期望 `<tmp>/.oneclaw/config.yaml` 存在且含 `openai:`；无需 API。

---

## Stub 编排提示

| 场景 | 典型 `Enqueue` 顺序 |
|------|---------------------|
| 纯回复 | `CompletionStop` ×1 |
| 单次工具 | `CompletionToolCalls` → `CompletionStop` |
| 写+读 | `ToolCalls(write)` → `ToolCalls(read)` → `CompletionStop`（或中间再 Enqueue 视 MaxSteps） |
| 多轮 model | 每个 step 消费队列中一项 |

实现新用例时：在表中将 `[ ]` 改为 `[x]` 并填写 **测试函数名**。
