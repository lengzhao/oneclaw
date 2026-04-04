# test/e2e

端到端测试：通过 **`OPENAI_BASE_URL`** 指向 `../openaistub` 的本地 HTTP 服务，**不调用真实 OpenAI**。

## 文档

- **[CASES.md](./CASES.md)**：全部 E2E 编号、前置条件、期望与 **实现状态表**（逐项实现时在此勾选）。

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
- 需要测 memory 时：使用临时 `HOME`，一般**不要**调用 `e2eEnvMinimal` 里的 `ONCLAW_DISABLE_MEMORY`（可用 `baseStubTransport` + 自行 `Setenv`）。

## 实现顺序建议

1. 会话闭环：E2E-02、E2E-04、E2E-05、E2E-81、E2E-82  
2. 工具路径：E2E-40～E2E-43  
3. Memory 注入：E2E-10～E2E-16  
4. MEMORY.md / recall：E2E-20～E2E-33  
5. daily log / transcript / 其它：E2E-50～E2E-61  

每完成一条，更新 **CASES.md** 中的状态列与测试函数名。
