# `oneclaw.json` 配置参考

运行时通过 `-config /path/to/oneclaw.json` 指定；若省略，则按顺序尝试：

1. `./oneclaw.json`
2. `$HOME/.oneclaw/oneclaw.json`

存在时才加载。

## 设计意图

配置的目标不是把所有能力都显式暴露给使用者，而是为宿主提供**默认可进化、按需增强**的运行时开关。

其中与默认自进化关系最直接的配置是：

- `workspace`
- `skills`
- `compaction`
- `background_agent`

## 敏感项环境变量覆盖

- `OPENAI_API_KEY` → `openai.api_key`
- `OPENAI_BASE_URL` → `openai.base_url`

## 示例

```json
{
  "openai": {
    "api_key": "",
    "base_url": "",
    "model": "gpt-4o-mini",
    "max_tokens": 2048
  },
  "workspace": {
    "root": "/path/to/workspace"
  },
  "skills": {
    "dir": ""
  },
  "mcp": {
    "servers": [
      {
        "name": "fs",
        "enabled": true,
        "command": "npx",
        "args": ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"],
        "env": {}
      }
    ]
  },
  "compaction": {
    "enabled": true,
    "max_turns_before_compact": 40,
    "keep_recent_turns": 12,
    "summary_model": ""
  },
  "background_agent": {
    "enabled": true,
    "system_prompt": "从对话中提取事实、经验与可改进点，用 markdown 三个小节输出。",
    "model": "",
    "debounce": "30s",
    "interval": "15m",
    "output_file": "memory/insights.md",
    "recent_turns": 30
  }
}
```

## 字段说明

| 块 | 字段 | 说明 |
|----|------|------|
| `openai` | `model` | 默认 `gpt-4o-mini` |
| `workspace` | `root` | 工作区根目录，可包含 `IDENTITY.md`、`SOUL.md`、`AGENTS.md`、`USER.md` |
| `skills` | `dir` | 非空则仅从该目录加载 `*.md`；为空且 `workspace.root` 有值时，默认使用 `<root>/skills` |
| `mcp.servers[]` | `enabled` | 必须为 `true` 才会启动该 stdio 服务 |
| `mcp.servers[]` | `command` / `args` | 子进程命令行；工具在模型侧名为 `mcp__<sanitized_name>__<toolName>` |
| `compaction` | `enabled` | 开启上下文压缩 |
| `compaction` | `max_turns_before_compact` | 超过阈值后，用 LLM 摘要旧内容 |
| `compaction` | `keep_recent_turns` | 摘要时保留最近若干 turn |
| `compaction` | `summary_model` | 为空则使用主 `openai.model` |
| `background_agent` | `enabled` | 开启后台分析 |
| `background_agent` | `debounce` | 每轮成功后防抖触发分析 |
| `background_agent` | `interval` | 额外定时触发；空字符串表示仅防抖 |
| `background_agent` | `output_file` | 追加写入的分析结果；相对路径相对 `workspace.root`，否则相对进程工作目录 |

## 与默认自进化的关系

### `workspace`

用于承载项目人格、静态知识与部分组织约束，是最直接的外部化知识入口。

推荐目录结构与文件职责见 [工作区布局](./workspace-layout.md)。

### `skills`

用于加载可复用能力说明和流程约束。默认推荐通过目录组织，而不是把所有规则硬编码到宿主。

### `compaction`

用于降低上下文长度压力，让对话记忆在较长会话中仍可持续使用。

### `background_agent`

适合生成低风险的分析产物，例如事实、经验、改进点；默认更适合作为“追加型沉淀”，不建议直接覆盖高敏感文件。

## CLI 覆盖

`-workspace`、`-skills` 可覆盖配置中的目录，便于本地调试与实验。

## MCP 工具命名

与静态 `tools.Registry` 并存时，MCP 工具带前缀，避免冲突。详见实现中的 `tools.Merged` 与 `tools.MCPServer`。

## 相关文档

- [默认自进化能力](../concepts/default-evolution.md)
- [工作区布局](./workspace-layout.md)
- [ADR-001：模块边界与接口形态](../architecture/adr-001-module-boundaries.md)
- [运维速查与排错](./runbook-troubleshooting.md)
