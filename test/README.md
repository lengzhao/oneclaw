# 集成 / E2E 测试（Stub 模型）

## 思路

- 使用 **`test/openaistub`** 在本地起 **HTTP 服务**，实现 **`POST /v1/chat/completions`**，返回与 OpenAI 兼容的 JSON。
- 使用 **`openai.NewClient(stubOpenAIOptions(stub)...)`**（见 `test/e2e/helpers_test.go`）把 **BaseURL** 与测试用 **API key** 注入客户端；**`baseStubTransport`** 将 **`rtopts`** 的 **`chat.transport`** 设为 **`non_stream`**（stub 只实现非流式，避免 `auto` 先走流式再回退时双次 dequeue）。

## 运行

```bash
go test ./test/e2e/... -count=1
```

用例清单与实现进度见 **[e2e/CASES.md](./e2e/CASES.md)**。

## 扩展

- 在 `stub.Enqueue(...)` 中按调用顺序放入多段 JSON，模拟多轮 model ↔ tool。
- 使用 `openaistub.CompletionStop` / `CompletionToolCalls` / `ToolCall` 拼响应；复杂场景可直接 `json.Marshal` 自定义 body。
