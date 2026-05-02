# E2E 用例清单（Stub 模型）

所有用例使用 **`go test -tags=e2e`**；默认 **`test/openaistub`** 与 **`rtopts`** 的 **`chat.transport: non_stream`**。需要用户级数据时 **`t.Setenv("HOME", tmpHome)`** 再写 `~/.oneclaw`。

**状态**：`[x]` 表示 `test/e2e` 下已有对应测试函数。

---

## 实现总表

| ID | 摘要 | 测试函数 · 文件 |
|----|------|-----------------|
| E2E-01 | 最小对话（纯文本结束） | `TestE2E_StubTextReply` · `stub_loop_test.go` |
| E2E-02 | 同 session 多轮 | `TestE2E_02_MultiTurnSameSession` · `stub_session_test.go` |
| E2E-03 | 工具调用闭环（read） | `TestE2E_StubToolThenText` · `stub_loop_test.go` |
| E2E-04 | 写后读 | `TestE2E_04_WriteThenRead` · `stub_session_test.go` |
| E2E-05 | Abort（ctx cancel） | `TestE2E_05_AbortCanceledContext` · `stub_session_test.go` |
| E2E-10～16 | 用户/项目 `AGENT.md`、rules 注入、`disable_memory`、`HOME` 降级 | `stub_memory_engine_test.go` |
| E2E-40～43 | 写路径、`grep` 根 | `stub_paths_test.go` |
| E2E-60～61 | transcript 往返与损坏 JSON | `stub_transcript_test.go` |
| E2E-70 | 出站助手文本 | `TestE2E_70_PublishOutboundAssistantText` · `stub_sink_test.go` |
| E2E-81～82 | 空入站、未知工具 | `stub_negative_test.go` |
| E2E-90～91 | `run_agent` / `fork_context` | `stub_subagent_test.go` |
| E2E-103～104 | 语义 compact 开/关 | `stub_semantic_compact_e2e_test.go` |
| E2E-105～107 | Skills 索引与 `invoke_skill` | `stub_skills_e2e_test.go` |
| E2E-108～109 | `tasks.json` 与 task 工具 | `stub_tasks_e2e_test.go` |
| E2E-111～112 | `cron` 与 scheduled jobs 段 | `stub_schedule_e2e_test.go` |
| E2E-113～118 | 本地斜杠、`Inbound` 元数据与附件 | `stub_inbound_orchestration_e2e_test.go` |
| E2E-116 | Agent 目录与 `run_agent` | `stub_agents_catalog_e2e_test.go` |
| E2E-130 | Eino 主路径：纯文本 `SubmitUser`（ADK → openaistub） | `TestE2E_StubEinoRuntime_SubmitUser` · `stub_eino_runtime_e2e_test.go` |
| E2E-131 | Eino 主路径：工具闭环 `read_file`（对齐 E2E-03） | `TestE2E_StubEinoRuntime_ToolReadFile` · `stub_eino_runtime_e2e_test.go` |
| E2E-132 | Eino 主路径：`run_agent` 嵌套（对齐 E2E-90） | `TestE2E_StubEinoRuntime_RunAgentNested` · `stub_eino_runtime_e2e_test.go` |
| E2E-133 | Eino 主路径：`fork_context`（对齐 E2E-91） | `TestE2E_StubEinoRuntime_ForkContext` · `stub_eino_runtime_e2e_test.go` |

---

## Stub 编排提示

| 场景 | 典型 `Enqueue` 顺序 |
|------|---------------------|
| 纯回复 | `CompletionStop` ×1 |
| 单次工具 | `CompletionToolCalls` → `CompletionStop` |
| 写+读 | `ToolCalls(write)` → `ToolCalls(read)` → `CompletionStop` |

实现新用例时在本表增加一行并运行 **`go test -tags=e2e ./test/e2e/...`**。
