# oneclaw

用 **Go** 实现的自主 Agent **运行时内核**：会话隔离、总线、ReAct 循环、工具与 Skills、可选工作区 Markdown（与 PicoClaw 布局概念对齐）。**不是** OpenClaw Gateway 的替代品，也不在首版追求与其控制面协议兼容。

设计说明与范围决策见 **`docs/`**（从 [文档入口](./docs/README.md) 读起；想快速跑通默认体验，先看 [快速开始](./docs/reference/quickstart.md)）。

## 快速对照

| 维度 | oneclaw（本仓库） | OpenClaw（参考） |
|------|----------------|------------------|
| 定位 | 可嵌入的 Go 库 + 轻量 CLI | 自托管 **Gateway** + 多通道 + 控制面 |
| 协议 | 自有 API（演进中） | Gateway 协议、HTTP API、多通道适配 |
| 文档 | `docs/` | [docs.openclaw.ai](https://docs.openclaw.ai/) |

## 模块路径

```text
github.com/lengzhao/oneclaw
```

## 本地构建（简述）

```bash
go test ./...
go run ./cmd/oneclaw -h
```

配置示例见 [docs/reference/config-reference.md](./docs/reference/config-reference.md)；设置 `OPENAI_API_KEY` 并放置 `oneclaw.json`（或使用默认搜索路径）后即可走真实 OpenAI，而非 `Mock`。

日志约定：`log/slog`。包布局约定：不使用 Go `internal/` 目录（见 ADR-001）。
