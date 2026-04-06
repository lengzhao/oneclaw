# Memory 维护双入口设计（回合后 vs 定时）

本文约定：**回合后自动维护**与**定时自动维护**不仅在**开关**上分离，在**代码上也为两个独立入口**，便于各自演进 prompt、可见历史、预算与审计语义，而**不**再通过单一函数 + `Scheduled bool` 分支堆砌逻辑。

**状态**：双入口 + **分路径 system 模板**（`maintenance_system_post_turn` / `maintenance_system_scheduled`）+ **审计 `post_turn_maintain` / `scheduled_maintain`** + **回合后 `PostTurnInput` 快照** + **`maintain.post_turn` YAML → `ONCLAW_POST_TURN_*`** + **`features.disable_scheduled_maintenance`** + **`maintainloop`**（见 [embedded-maintain-scheduler-design.md](embedded-maintain-scheduler-design.md)）。远场维护通过 **`ScheduledMaintainOpts.ToolRegistry`** 注入只读工具（`cmd/maintain` / `maintainloop` 默认 `builtin.ScheduledMaintainReadRegistry`，避免 `memory` 包 import `builtin` 成环）。**待续**：定时路径可选 transcript 窄读。与 Claude Code 范式对照见 [claude-code-memory-system.md](claude-code-memory-system.md) §13 / §15。

---

## 1. 为什么需要两个入口

| 问题 | 单一 `RunMaintain(..., scheduled bool)` 的代价 |
|------|-----------------------------------------------|
| 可读性 | 分支随时间膨胀，难以区分「本回合」与「周期任务」的真实差异 |
| 产品语义 | 近场增量 vs 远场整理本应对齐不同输入范围与输出形态，布尔位表达力不足 |
| 测试 | 同一函数测两套行为，fixture 易混 |
| 演进 | 一方改 prompt 易误伤另一方 |

因此：**对外两个入口**；内部可复用**无业务歧义的纯函数**（如 bullet 去重、段追加、读 daily log 字节切片），但**不**复用「整段 LLM 维护流水线」的单一实现。

---

## 2. 两个入口（建议命名与调用方）

命名仅为建议，实现时可微调，但须保持 **两个顶层导出函数**（或等价：`Engine` 上两个方法），且**禁止**再向公共 API 暴露「维护类型」枚举混在一条路径里。

### 2.1 回合后维护（近场 / extract 取向）

| 项 | 约定 |
|----|------|
| **产品语义** | **仅本回合刚结束**可沉淀的内容：事实、规则、注意事项、工具使用偏好、同一工具多次调用及原因（须在本回合文本/轨迹中可推断）。**不**从其它会话挖新点；daily log 可能含多日多会话，prompt 明确以 **Current turn snapshot** 为主证。 |
| **建议符号** | `memory.MaybePostTurnMaintain`（门控 + 节流） / `memory.RunPostTurnMaintain`（实际执行一次） |
| **调用方** | `session.Engine` 在每轮成功 `SubmitUser` 后（与现有 `PostTurn` 写 daily log 之后衔接） |
| **开关** | `features.disable_auto_maintenance` / `ONCLAW_DISABLE_AUTO_MAINTENANCE` **仅**控制此入口 |
| **输入视野（当前实现）** | **仅当前回合**：**`PostTurnInput`** 快照（user / assistant / 工具轨迹与重复调用摘要）+ **`MEMORY.md` 摘录**（去重）；**不**读 daily log、**不**读 project topic。门控：`ONCLAW_POST_TURN_MAINTENANCE_MIN_LOG_BYTES` 作用于快照总字节（见 `docs/config.md`） |
| **输出（当前实现）** | 仍写同一 `## Auto-maintained (日期)`；审计 **`post_turn_maintain`**；日志 **`pathway=post_turn`** |

### 2.2 定时维护（远场 / dream 取向）

| 项 | 约定 |
|----|------|
| **产品语义** | **跨会话、跨天**的整体整理：去重/合并、更新或收紧规则、标注过时并由新事实**取代**；面向 `MEMORY.md` 与 topic 的**整体一致性**，而非单回合增量。 |
| **建议符号** | `memory.RunScheduledMaintain`（单次蒸馏）；`maintainloop` / `cmd/maintain` 只调此入口 |
| **调用方** | 主进程内嵌 interval 循环、`cmd/maintain`、未来独立 job |
| **开关** | **`features.disable_scheduled_maintenance`** / **`ONCLAW_DISABLE_SCHEDULED_MAINTENANCE`**（后台：进程内 loop + `cmd/maintain` 间隔；**不**挡 **`maintain -once`**） |
| **输入视野（当前实现）** | **interval 定时**：daily log **增量**（行时间戳 + `scheduled_maintain_state.json` 高水位；首次 lookback = interval）。**`-once` 或 `Interval==0`**：按 `ONCLAW_MAINTENANCE_LOG_DAYS` 做日历天 **log 体量探测**；正文由 Agent 经 **`opts.ToolRegistry` 只读工具**（如 `read_file` / `grep` / `glob` / `list_dir`）自读，user prompt 给绝对路径与任务说明。`ToolRegistry==nil` 则跳过远场。字节门控见 `docs/config.md` |
| **输出（当前实现）** | `## Auto-maintained (日期)`；审计 **`scheduled_maintain`**；**scheduled** system 模板；user prompt 强调 consolidation / supersede |

### 2.3 与「写 daily log」的关系

- **`memory.PostTurn`**：仍仅为 **daily log 追加**（信号层），**不是**维护入口之一。
- 两个维护入口均为 **LLM 维护流水线**，与 `PostTurn` 解耦、顺序上通常 **PostTurn 先、再 MaybePostTurnMaintain**（具体顺序以实现为准）。

### 2.4 本地 slash 旁路（不跑 PostTurn / 维护）

`session.Engine` 在识别为 **本地 slash 命令**时走 `submitLocalSlashTurn`：**不**经过 `loop.RunTurn`，**不**调用 `memory.PostTurn`、**不**调用 `memory.MaybePostTurnMaintain`。这是**刻意设计**，不是遗漏。

| 理由 | 说明 |
|------|------|
| 无模型回合 | 无 assistant 生成内容可供「本回合快照」蒸馏；工具轨迹为空或无关 |
| 信号质量 | 若对 slash 也追加 daily log 并触发近场维护，易把「固定帮助文案」等低价值文本写入维护输入 |
| 与定时维护的关系 | **定时维护**（`RunScheduledMaintain`）仍可按既有规则读 daily log；slash 未写入的回合**不会**出现在 log 中，符合「旁路不沉淀为可维护信号」的语义 |

若将来产品要求 slash 也参与观测或审计，应**单独**设计（例如只记 transcript、不触发 LLM 维护），而**不是**强行复用完整 `PostTurn` + `MaybePostTurnMaintain` 链。交叉说明见 [code-simplification-opportunities.md](code-simplification-opportunities.md) §2.2。

---

## 3. 共享与互斥

### 3.1 允许共享（库内部）

- 文件路径解析、`MEMORY.md` 物理追加、bullet 级去重、审计 `AppendMemoryAudit` 的底层辅助。
- `ResolveMaintenanceModel` 可拆为 **PostTurn** 与 **Scheduled** 两套包装函数，或各入口内联清晰条件，避免一个 `scheduled bool` 贯穿全包。

### 3.2 已分路径的细节

- **数据抓取范围**：**`distillConfig`**（post-turn vs scheduled）。
- **回合后近场**：**`PostTurnInput`** → **Current turn snapshot**（含 **tools** 与 **repeated_in_this_turn**；`ONCLAW_POST_TURN_MAINTAIN_*_SNAPSHOT_BYTES` 截断）；外加 **MEMORY.md 前缀**；**不**拼接 daily log / topic。
- **System 模板**：**`prompts.NameMaintenanceSystemPostTurn`** / **`NameMaintenanceSystemScheduled`**。
- **门控条件**：当日若已有 `## Auto-maintained (YYYY-MM-DD)`，**不跳过**维护运行；模型输出与当日块 **合并**（去重、保留旧条 + 新条），写回时 **替换**该日 span，避免重复标题段。
- **待续**：定时路径 **transcript** 窄读、**工具**白名单。

### 3.3 并发

两入口可能同进程交错执行：须 **全局互斥或单 worker** 序列化「会写 `MEMORY.md` / topic 的维护临界区」，避免与主会话其它写 memory 工具竞态（与 [embedded-maintain-scheduler-design.md](embedded-maintain-scheduler-design.md) §4 一致）。

---

## 4. 配置与文档

| 配置 / 文档 | 说明 |
|-------------|------|
| `disable_auto_maintenance` | 仅 **回合后** |
| `disable_scheduled_maintenance` | **后台定时**（`maintainloop`、`cmd/maintain` 间隔） |
| `maintain.post_turn.*` | 经 `ApplyEnvDefaults` → `ONCLAW_POST_TURN_MAINTENANCE_*`（见 `docs/config.md`） |
| `maintain.interval`（YAML 非空） | 启用 **`maintainloop`**；**仅 env** 不启进程内 loop |
| `maintain.model` / `scheduled_model` | **PostTurn** vs **Scheduled** 模型链 |

---

## 5. 迁移路径（记录）

**已完成**

1. 导出 **`RunScheduledMaintain`**；**`cmd/maintain`** 仅调用该入口。
2. 导出 **`RunPostTurnMaintain`**、**`MaybePostTurnMaintain`**；**`session.Engine`** 在 `PostTurn` 后调用 **`MaybePostTurnMaintain`**。
3. **`MaybeMaintain`** → 弃用别名，转发 **`MaybePostTurnMaintain`**。
4. 移除公共 **`MaintainOptions` / `RunMaintain`**；内部统一为 **`runDistill` + `distillConfig` + `maintainPathway`**。
5. **`maintainPipelineMu`**：两路径互斥执行蒸馏写盘临界区。
6. **System 模板**、**审计 source**、**PostTurn 快照**、**YAML post_turn**、**disable_scheduled_maintenance**、**maintainloop**。

**后续迭代**

- 定时路径 **transcript** 窄读；两路径 **工具**可配置注册表。

---

## 6. 与内嵌调度文档的关系

主进程 **interval 协程**只负责唤醒 **定时入口**；详见 [embedded-maintain-scheduler-design.md](embedded-maintain-scheduler-design.md)。该文档中的函数名在双入口落地后应统一为 **`RunScheduledMaintain`**（而非泛化 `RunMaintain`）。

---

## 7. 小结

| 原则 | 说明 |
|------|------|
| 两个公共入口 | 回合后一条、定时一条；开关独立 |
| 少共享流水线 | 共享底层 IO/去重即可，prompt 与取数范围分开 |
| 审计可区分 | **`post_turn_maintain` / `scheduled_maintain`**（`AppendMemoryAudit`） |
| 互斥 | 写 memory 临界区全局串行化 |

---

*实现：`memory/maintain_run.go`、`memory/maintain.go`、`memory/maintain_turn_snapshot.go`、`session/engine.go`、`cmd/maintain/main.go`、`maintainloop/`、`prompts/templates/maintenance_system_*.tmpl`。*
