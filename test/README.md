# 集成 / E2E 测试（Stub 模型）

## 思路

- 使用 **`test/openaistub`** 在本地起 **HTTP 服务**，实现 **`POST /v1/chat/completions`**，返回与 OpenAI 兼容的 JSON。
- **`Engine.EinoOpenAIAPIKey` / `EinoOpenAIBaseURL`** 指向 **`test/openaistub`**（见 `test/e2e/helpers_test.go` 的 **`stubOpenAIOptions`** / **`newStubEngine`**）；**`baseStubRtopts`** 在每测重置 **`rtopts`**，避免与其它用例的 **`PushRuntime`** 快照串扰。

## 运行

```bash
go test ./test/e2e/... -count=1
```

用例清单与实现进度见 **[e2e/CASES.md](./e2e/CASES.md)**。

## 扩展

- 在 `stub.Enqueue(...)` 中按调用顺序放入多段 JSON，模拟多轮 model ↔ tool。
- 使用 `openaistub.CompletionStop` / `CompletionToolCalls` / `ToolCall` 拼响应；复杂场景可直接 `json.Marshal` 自定义 body。
