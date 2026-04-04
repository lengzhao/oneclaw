# 集成 / E2E 测试（Stub 模型）

## 思路

- 使用 **`test/openaistub`** 在本地起 **HTTP 服务**，实现 **`POST /v1/chat/completions`**，返回与 OpenAI 兼容的 JSON。
- **`openai.NewClient()`** 已通过官方 SDK 的 **`OPENAI_BASE_URL`** 指向 stub 的根路径（含 `/v1/`），**无需改业务代码**。
- 测试里设置：
  - `OPENAI_BASE_URL=<stub.BaseURL()>`（例如 `http://127.0.0.1:12345/v1/`）
  - `OPENAI_API_KEY` 任意非空（stub 不校验）
  - **`ONCLAW_CHAT_TRANSPORT=non_stream`**（stub 只实现非流式；默认 `auto` 会先走流式，需避免）

## 运行

```bash
go test ./test/e2e/... -count=1
```

用例清单与实现进度见 **[e2e/CASES.md](./e2e/CASES.md)**。

## 扩展

- 在 `stub.Enqueue(...)` 中按调用顺序放入多段 JSON，模拟多轮 model ↔ tool。
- 使用 `openaistub.CompletionStop` / `CompletionToolCalls` / `ToolCall` 拼响应；复杂场景可直接 `json.Marshal` 自定义 body。
