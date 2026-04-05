# 统一配置（config 包）

开发与生产共用同一套加载规则：`github.com/lengzhao/oneclaw/config`。

## 配置文件路径与合并顺序

从低到高优先级（后者覆盖前者）：

1. **用户级**：`~/.oneclaw/config.yaml`
2. **项目级**：`<cwd>/.oneclaw/config.yaml`
3. **显式文件**：`oneclaw -config /path/to.yaml` 或 `maintain -config /path/to.yaml`（相对路径相对于 `-cwd` / 当前进程的 cwd）

缺失的文件会被忽略；若 `-config` 指向的路径不存在，启动报错。

## 敏感项（API Key）

- 推荐在 YAML 中配置 `openai.api_key`，由 `openai.NewClient(config.OpenAIOptions()...)` 注入，**不**依赖把 `OPENAI_API_KEY` 写进进程环境，减少子进程/脚本继承环境导致泄漏的风险。
- 若合并后的 YAML 未提供 key，仍可使用环境变量 `OPENAI_API_KEY`（便于本地与 CI）。
- 当 YAML 中配置了非空的 `openai.api_key` 时，其优先级**高于**环境变量中的 `OPENAI_API_KEY`（文件为主真源）。

## 非敏感项与环境变量

下列项在 **环境变量已设置** 时以环境为准（便于临时覆盖）；否则使用合并后的 YAML；再否则沿用各包原有默认值。

| 区域 | YAML 字段 | 常见环境变量 |
|------|-----------|----------------|
| 模型 | `model` | `ONCLAW_MODEL` |
| 传输 | `chat.transport` | `ONCLAW_CHAT_TRANSPORT` |
| Base URL | `openai.base_url` | `OPENAI_BASE_URL`（环境优先于文件） |
| 组织 / 项目 | `openai.org_id`、`openai.project_id` | `OPENAI_ORG_ID`、`OPENAI_PROJECT_ID`（环境优先于文件） |
| 路径 | `paths.*` | `ONCLAW_MEMORY_BASE`、`ONCLAW_TRANSCRIPT_PATH` 等 |
| 预算 | `budget.*` | `ONCLAW_MAX_PROMPT_BYTES`、`ONCLAW_MIN_TRANSCRIPT_MESSAGES`；语义 compact：`ONCLAW_COMPACT_SUMMARY_MAX_BYTES`、`ONCLAW_DISABLE_SEMANTIC_COMPACT` |
| 维护 | `maintain.*` | `ONCLAW_MAINTAIN_INTERVAL`、`ONCLAW_MAINTENANCE_MODEL`；多日 log / topic：`ONCLAW_MAINTENANCE_LOG_DAYS`、`ONCLAW_MAINTENANCE_MAX_COMBINED_LOG_BYTES`、`ONCLAW_MAINTENANCE_MAX_TOPIC_FILES`、`ONCLAW_MAINTENANCE_TOPIC_EXCERPT_BYTES` |
| 日志 | `log.*` | `ONCLAW_LOG_LEVEL`、`ONCLAW_LOG_FORMAT` |
| 开关 | `features.disable_*` | 对应 `ONCLAW_DISABLE_*`（见示例文件） |

启动时若调用了 `config.ApplyEnvDefaults`，会把「当前仍为空的」`ONCLAW_*` 设为 YAML 中的值，使 `memory`、`budget` 等仍读环境的代码与文件配置一致；**不会**设置 `OPENAI_API_KEY`。

## 示例

仓库内模板：`.oneclaw/config.example.yaml`。

## 与第三方 autoload 的关系

入口仍可保留 `_ "github.com/lengzhao/conf/autoload"`，用于 `.env` 等；与 YAML 合并规则独立。若同一键既在 env 又在 YAML，以上述「敏感 / 非敏感」规则为准。
