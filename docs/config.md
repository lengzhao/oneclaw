# 统一配置（config 包）

开发与生产共用同一套加载规则：`github.com/lengzhao/oneclaw/config`。

## 配置文件路径与合并顺序

从低到高优先级（后者覆盖前者）：

1. **用户级**：`~/.oneclaw/config.yaml`
2. **项目级**：`<cwd>/.oneclaw/config.yaml`
3. **显式文件**：`oneclaw -config /path/to.yaml` 或 `maintain -config /path/to.yaml`（相对路径相对于 `-cwd` / 当前进程的 cwd）

缺失的文件会被忽略；若 `-config` 指向的路径不存在，启动报错。

**初始化项目**：`oneclaw -init`（可选 `-cwd <dir>`）在 `<cwd>/.oneclaw/config.yaml` **不存在** 时写入内置模板（`config` 包嵌入的 `project_init.example.yaml`，应与根目录 `config.example.yaml` 保持同步），并创建记忆目录；**不**覆盖已有 `config.yaml`。

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
| 维护 | `maintain.*` | **定时** `RunScheduledMaintain`：`ONCLAW_MAINTAIN_INTERVAL`、`ONCLAW_MAINTENANCE_*`；**interval 模式**下 daily log 为**增量**（行内时间戳 + `.oneclaw/scheduled_maintain_state.json`）。**`-once` 或 `Interval==0`** 仍用按天 `LOG_DAYS`（远场为多步、只读工具，须在 `opts.ToolRegistry` 传入如 `builtin.ScheduledMaintainReadRegistry()`；`ToolRegistry==nil` 则跳过）。**回合后**见下节 `ONCLAW_POST_TURN_*`。双入口见 [memory-maintain-dual-entry-design.md](memory-maintain-dual-entry-design.md) |
| 日志 | `log.*` | `ONCLAW_LOG_LEVEL`、`ONCLAW_LOG_FORMAT`；默认另写 `<cwd>/.oneclaw/logs/YYYY/MM/oneclaw-<时间戳>.log`，`ONCLAW_DISABLE_LOG_FILE=1` 关闭仅写 stderr |
| 开关 | `features.disable_*` | 对应 `ONCLAW_DISABLE_*`（见示例文件）；**`disable_scheduled_maintenance`** → `ONCLAW_DISABLE_SCHEDULED_MAINTENANCE`（关闭 **进程内 maintainloop** 与 **`cmd/maintain` 的 interval 循环**；**不**影响 **`oneclaw -maintain-once`** / **`maintain -once`**） |
| Skills | — | `ONCLAW_DISABLE_SKILLS=1` 关闭系统提示里的 Skills 索引（`invoke_skill` 仍可用）；`ONCLAW_SKILLS_RECENT_PATH` 覆盖 `<cwd>/.oneclaw/skills-recent.json`；`ONCLAW_SKILLS_INDEX_MAX_BYTES` 覆盖索引字节上限（默认约为 `ONCLAW_MAX_PROMPT_BYTES` 的 1%，见 `budget.Global.SkillIndexMaxBytes`） |
| 任务列表 | — | `ONCLAW_DISABLE_TASKS=1` 关闭系统提示里的任务摘要，并拒绝 `task_create` / `task_update`；任务文件默认为 `<cwd>/.oneclaw/tasks.json` |
| 定时任务（agent） | — | `ONCLAW_DISABLE_SCHEDULED_TASKS=1` 关闭系统提示里的 Scheduled jobs 段，并拒绝 `cron` 工具；数据文件 `<cwd>/.oneclaw/scheduled_jobs.json`；调度按**下一触发时间**睡眠（`schedule.NextWakeDuration`），`add`/`remove`/到期执行后会 **notify** 唤醒；`ONCLAW_SCHEDULE_MIN_SLEEP`（Go duration，默认 **1s**，避免过短睡眠空转）、`ONCLAW_SCHEDULE_IDLE_SLEEP`（当前 channel 实例无下一触发点时，默认 **1h**，仍可通过变更任务唤醒） |
| 行为策略写回 | — | `ONCLAW_DISABLE_BEHAVIOR_POLICY_WRITE=1` 拒绝 `write_behavior_policy`。该工具**不在**默认主会话 `DefaultRegistry` 中，由 **`RunScheduledMaintain` 使用的 registry**（如 `builtin.ScheduledMaintainReadRegistry`）等场景注册；仅允许写入**当前 cwd** 下 `<cwd>/.oneclaw/rules/*.md`、`<cwd>/.oneclaw/AGENT.md`、`<cwd>/.oneclaw/skills/<name>/SKILL.md`、`<cwd>/.oneclaw/memory/MEMORY.md`（目标名 `rule` / `skill` / `agent_md` / `memory`），**不可**写用户目录 `~/.oneclaw/*`；写入记入 D2 审计（`source=write_behavior_policy`） |
| 侧链合并 | — | `ONCLAW_SIDECCHAIN_MERGE`：留空关闭；`1` / `true` / `tool` / `append` 在 `run_agent` / `fork_context` 的 **tool 结果**末尾附加侧链文件路径；`user` 则在同一轮工具输出之后向主 transcript 追加一条 **user** 消息（摘要 + 路径） |

启动时若调用了 `config.ApplyEnvDefaults`，会把「当前仍为空的」`ONCLAW_*` 设为 YAML 中的值，使 `memory`、`budget` 等仍读环境的代码与文件配置一致；**不会**设置 `OPENAI_API_KEY`。

### 维护：`ONCLAW_POST_TURN_*`（回合后）与 `ONCLAW_MAINTENANCE_*`（定时）

| 环境变量 | 适用路径 | 说明 |
|----------|----------|------|
| `ONCLAW_POST_TURN_MAINTENANCE_MIN_LOG_BYTES` | 回合后 | 格式化后的 **Current turn snapshot**（UTF-8）低于此字节则跳过近场维护，默认 **200** |
| `ONCLAW_POST_TURN_MAINTENANCE_MEMORY_PREVIEW_BYTES` | 回合后 | 注入 prompt 的 `MEMORY.md` 前缀上限（仅用于去重参考），默认 **4000** |
| `ONCLAW_POST_TURN_MAINTENANCE_TIMEOUT_SEC` | 回合后 | 单次蒸馏超时，默认 **60** |
| `ONCLAW_POST_TURN_MAINTENANCE_MAX_TOKENS` | 回合后 | 若设置则作为该路径 completion 上限（与引擎 `MaxTokens` 协同） |
| `ONCLAW_MAINTENANCE_LOG_DAYS` 等 | **定时** | **`oneclaw -maintain-once` / `maintain -once` 或 `RunScheduledMaintain(..., opts)` 且 `opts.Interval==0`**：按**日历天**做 log 体量探测（正文由 Agent 经只读工具自读）。**`maintainloop` / `cmd/maintain -interval > 0`**：改用**增量模式**（见下），`LOG_DAYS` 不参与取数 |
| `ONCLAW_MAINTENANCE_INCREMENTAL_OVERLAP` | **定时（interval）** | 增量高水位回退重叠，Go duration，默认 **2m**（防时钟/顺序边界漏行） |
| `ONCLAW_MAINTENANCE_INCREMENTAL_MAX_SPAN` | **定时（interval）** | 单次取数最早不早于 `now -` 此值，默认 **168h**（停机过久避免一次塞满） |
| `ONCLAW_SCHEDULED_MAINTENANCE_TIMEOUT_SEC` | **定时** | 默认 **1800**（30m，多步只读工具 + 慢 API）；最大 **3600** |
| `ONCLAW_DISABLE_SCHEDULED_MAINTENANCE` | **定时（后台）** | 为 `1`/`true` 时：`maintainloop` 不启动、`cmd/maintain` 间隔循环立即退出；**`oneclaw -maintain-once` / `maintain -once` 仍执行** |
| `ONCLAW_POST_TURN_MAINTAIN_USER_SNAPSHOT_BYTES` | 回合后 | 注入维护 user prompt 的本轮 **user** 文本上限，默认 **4000** |
| `ONCLAW_POST_TURN_MAINTAIN_ASSISTANT_SNAPSHOT_BYTES` | 回合后 | 本轮 **assistant** 可见文本上限，默认 **8000** |

**YAML `maintain.post_turn.*`**（非 0 / 非空时经 `ApplyEnvDefaults` 写入对应 `ONCLAW_POST_TURN_MAINTENANCE_*`）：**生效项**为 `min_log_bytes`、`memory_preview_bytes`、`timeout_seconds`、`max_tokens`。`log_days`、`max_combined_log_bytes`、`max_log_bytes`、`max_topic_files`、`topic_excerpt_bytes` 仍可写入环境，但**近场不再读取**（无 daily log / topic 注入）；多日整理请用 **定时** `RunScheduledMaintain` 与 `ONCLAW_MAINTENANCE_*`。

**定时 incremental（interval）**：`maintainloop` 与 **`cmd/maintain -interval <dur>`** 调用 `RunScheduledMaintain` 时传入 **`ScheduledMaintainOpts{ Interval, ToolRegistry }`**（`ToolRegistry` 与 **单次远场**（`-maintain-once` / `-once`）相同，见 `tools/builtin.ScheduledMaintainReadRegistry`）。Daily log 按行首 RFC3339 时间筛选：**首次**为 `(now - Interval, now]`；之后为「**上次成功远场维护**写入状态里的 **`high_water_log_utc`** 减 overlap」之后到 `now` 的行。成功写入当日 **`memory/YYYY-MM-DD.md`**（episodic digest，与 `MEMORY.md` 同目录）或「去重后无新 bullet」仍会推进高水位，避免反复喂同一批 log。（根上 **`MEMORY.md`** 仅承载**规则**，不由维护流水线追加 episodic 段。）**状态路径**：`<cwd>/.oneclaw/scheduled_maintain_state.json`（与仓库 cwd 绑定；**不**放在 `<cwd>/.oneclaw/memory/` 下）。若曾使用旧版本将状态写在 `memory/.oneclaw/` 下，进程会在首次定时维护时自动迁移到上述路径。

**远场多步**：`ONCLAW_SCHEDULED_MAINTENANCE_MAX_STEPS`（默认 **24**，范围 2–64）限制模型↔只读工具轮数；user prompt 仅给路径与任务说明，不内嵌全文 log。

**日志量过大**：仍受 `ONCLAW_MAINTENANCE_MAX_COMBINED_LOG_BYTES`、`ONCLAW_MAINTENANCE_MAX_LOG_BYTES` 等上限影响（用于探测与边界）；极端情况可调小 interval 或调低 `MAX_STEPS`。

**进程内 `maintainloop`**（`cmd/oneclaw`）：仅当合并后的 YAML 里 **`maintain.interval` 非空** 时启用，间隔取 `EmbeddedScheduledMaintainInterval()`（与 `MaintainLoopInterval` 解析一致）。仅设置环境变量 `ONCLAW_MAINTAIN_INTERVAL` 而**不写** YAML `interval` **不会**启动进程内循环（仍可用 **`cmd/maintain`** 做间隔循环，或 **`oneclaw -maintain-once`** 做按天单次）。

回合后开关：**`features.disable_auto_maintenance`** / `ONCLAW_DISABLE_AUTO_MAINTENANCE`。**定时后台**开关：上表 `disable_scheduled_maintenance`。详见 [embedded-maintain-scheduler-design.md](embedded-maintain-scheduler-design.md)。

## 示例

仓库内模板：根目录 [`config.example.yaml`](../config.example.yaml)。**`oneclaw -init`** 使用的嵌入副本为 [`config/project_init.example.yaml`](../config/project_init.example.yaml)（请与前者保持内容同步）。

## 与第三方 autoload 的关系

入口仍可保留 `_ "github.com/lengzhao/conf/autoload"`，用于 `.env` 等；与 YAML 合并规则独立。若同一键既在 env 又在 YAML，以上述「敏感 / 非敏感」规则为准。
