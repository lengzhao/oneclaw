# test/e2e

端到端测试：通过 **`stubOpenAIOptions(stub)`** 将客户端指向 `../openaistub` 的本地 HTTP 服务，**不调用真实 OpenAI**。

## 文档

- **[CASES.md](./CASES.md)**：全部 E2E 编号、前置条件、期望与 **实现状态表**（逐项实现时在此勾选）；默认 **openaistub**，供 CI 使用。
- **[CASES_LIVE.md](./CASES_LIVE.md)**：调用 **真实 LLM** 时专验 **自学/进化**（memory 落盘、recall、维护、compact 等在真模型下的效果）；**异常与边界**仍以 Stub **[CASES.md](./CASES.md)** 为准；含「预期 vs 实际差异」记录方式；不默认进 CI。

## 测试文件

| 文件 | 内容 |
|------|------|
| `helpers_test.go` | `concatUserText`、`newStubEngine`、`baseStubTransport` / `e2eEnv*` |
| `stub_loop_test.go` | E2E-01、03 |
| `stub_session_test.go` | E2E-02、04、05 |
| `stub_negative_test.go` | E2E-81、82 |
| `stub_paths_test.go` | E2E-40～43 |
| `stub_memory_engine_test.go` | E2E-10～16、30、31（`session.SubmitUser` + memory） |
| `stub_recall_more_test.go` | E2E-32、33（recall 去重与预算） |
| `stub_sink_test.go` | E2E-70（`SinkRegistry` + `KindText`/`KindDone`） |
| `stub_memory_bundle_test.go` | E2E-20～22、52（`memory.BuildTurn`） |
| `stub_transcript_test.go` | E2E-60、61 |
| `stub_postturn_test.go` | E2E-50、51 |
| `stub_maintain_test.go` | E2E-92 |
| `stub_maintain_pipeline_e2e_test.go` | E2E-101、102、113（近场仅快照；去重；远场多日 log+topic） |
| `stub_semantic_compact_e2e_test.go` | E2E-103、104（全局预算语义 compact） |
| `stub_audit_test.go` | E2E-93～95 |
| `stub_maintain_cli_test.go` | E2E-96、97 |
| `stub_schedule_e2e_test.go` | E2E-111、112 |

`../openaistub` 会记录每次 `POST /v1/chat/completions` 的 JSON 体（`ChatRequestBodies` / `ChatRequestUserTextConcat`），供 E2E-101 等断言 prompt。

## 运行

```bash
go test ./test/e2e/... -count=1
```

单测：

```bash
go test ./test/e2e/... -run TestE2E_StubTextReply -count=1 -v
```

## 公共约定

- **`helpers_test.go`**：`baseStubTransport`、`e2eEnvMinimal` 等（见文件内注释）。
- 需要测 memory 时：使用临时 `HOME`，一般**不要**用 `e2eEnvMinimal`（其默认 `DisableMemory=true`）；改用 **`e2eEnvWithMemory`** 等。

## 实现顺序建议

1. 会话闭环：E2E-02、E2E-04、E2E-05、E2E-81、E2E-82  
2. 工具路径：E2E-40～E2E-43  
3. Memory 注入：E2E-10～E2E-16  
4. MEMORY.md / recall：E2E-20～E2E-33  
5. daily log / transcript / 其它：E2E-50～E2E-61  

每完成一条，更新 **CASES.md** 中的状态列与测试函数名。

## 真实 LLM（opt-in）

- 配置：复制 `live_llm.config.example.yaml` → `live_llm.config.yaml`，填写 `openai.api_key`（后者已 `.gitignore`）。
- 运行：`go test -tags=live_llm ./test/e2e/... -run TestLiveLLM -count=1 -v`
- 说明：见 **[CASES_LIVE.md](./CASES_LIVE.md)**；无 `live_llm.config.yaml` 时测试会 `Skip`。
