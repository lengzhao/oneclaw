# test/e2e

端到端测试：通过 **`stubOpenAIOptions(stub)`** 将客户端指向 `../openaistub` 的本地 HTTP 服务，**不调用真实 OpenAI**。构建需加 **`//go:build e2e`**。

## 文档

- **[CASES.md](./CASES.md)**：当前 Stub 用例与测试函数对照表。
- **[CASES_LIVE.md](./CASES_LIVE.md)**：可选 **真实 LLM**（`-tags=live_llm`）；不默认进 CI。

## 测试文件（当前）

| 文件 | 内容 |
|------|------|
| `helpers_test.go` | `e2eEnvMinimal`、`e2eEnvWithMemory`、`e2eWaitForFile` 等 |
| `stub_loop_test.go` | 最小对话、工具闭环 |
| `stub_session_test.go` | 多轮、写后读、Abort |
| `stub_negative_test.go` | 空入站、未知工具 |
| `stub_paths_test.go` | 写路径、`grep` |
| `stub_memory_engine_test.go` | `AGENT.md` / rules 注入、`disable_memory` |
| `stub_transcript_test.go` | transcript 往返 |
| `stub_sink_test.go` | 出站助手文本 |
| `stub_semantic_compact_e2e_test.go` | 语义 compact |
| `stub_skills_e2e_test.go` | Skills |
| `stub_tasks_e2e_test.go` | tasks 与 task 工具 |
| `stub_schedule_e2e_test.go` | `cron` / `scheduled_jobs.json` |
| `stub_inbound_orchestration_e2e_test.go` | 本地斜杠、入站元数据与附件 |
| `stub_agents_catalog_e2e_test.go` | Agent 目录与 `run_agent` |
| `stub_subagent_test.go` | `run_agent` / `fork_context` |
| `live_llm_e2e_test.go` | 真模型（单独 build tag） |

`../openaistub` 可记录 `POST /v1/chat/completions` 的请求体，供断言使用。

## 运行

```bash
go test -tags=e2e ./test/e2e/... -count=1
```

单测：

```bash
go test -tags=e2e ./test/e2e/... -run TestE2E_StubTextReply -count=1 -v
```

## 公共约定

- 需要测指令注入时：临时 `HOME` + **`e2eEnvWithMemory`**（`e2eEnvMinimal` 常默认 `DisableMemory=true`）。

## 真实 LLM（opt-in）

- 配置：复制 `live_llm.config.example.yaml` → `live_llm.config.yaml`（已 `.gitignore`）。
- 运行：`go test -tags='e2e live_llm' ./test/e2e/... -run TestLiveLLM -count=1 -v`
- 说明：**[CASES_LIVE.md](./CASES_LIVE.md)**。
