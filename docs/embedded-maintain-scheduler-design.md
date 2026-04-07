# 主进程内嵌定时维护（设计）

本文描述 **`oneclaw` 主进程**中的 **`maintainloop`**：在合并 YAML 中**显式配置** `maintain.interval` 时，按间隔调用 **`RunScheduledMaintain`**（见 [memory-maintain-dual-entry-design.md](memory-maintain-dual-entry-design.md)）。

**状态**：**已实现**（`maintainloop` 包 + `config.Resolved.EmbeddedScheduledMaintainInterval`）。

## 1. 背景与目标

### 1.1 行为

- **主进程**：`cmd/oneclaw` 在 `StartAll` 之前调用 **`maintainloop.Start`**；若 **`EmbeddedScheduledMaintainInterval() > 0`** 且未禁用后台定时，则 **立即** 跑一次 **`RunScheduledMaintain`**（带 **`ScheduledMaintainOpts{Interval}`** → **增量 daily log**），再按间隔重复。
- **启用条件**：合并 YAML 中 **`maintain.interval` 必须非空**（未写则不启动进程内 loop，避免默认误启；**`cmd/maintain`** 可用其自己的默认间隔做循环）。
- **`oneclaw -maintain-once`**：单次 **`RunScheduledMaintain`**（`Interval==0`）→ **按天 `LOG_DAYS`** 窗口，适合 crontab。**`cmd/maintain`**：interval 循环（传 **Interval** → 增量 log）；**`-once`** 同上按天窗口。若 **`features.disable_scheduled_maintenance`** 为真则**不进入**间隔循环（**单次远场仍执行**）。

### 1.2 目标（验收）

- 与 **回合后** `MaybePostTurnMaintain` **互斥写盘**（`memory.maintainPipelineMu`）。
- **`features.disable_scheduled_maintenance`** 关闭进程内 loop 与 **maintain 间隔模式**。

## 2. 架构

`cmd/oneclaw/main.go`：`Load` + `PushRuntime` 后调用

`maintainloop.Start(ctx, Params{ Interval: cfg.EmbeddedScheduledMaintainInterval(), Layout, Client, MainModel, MaxMaintainTokens })`。

`maintainloop` 若 `Interval <= 0`、`Client == nil`、或 **`memory.ScheduledMaintenanceBackgroundDisabled()`** 则 **no-op**。

## 3. 配置摘要

| 项 | 说明 |
|----|------|
| `maintain.interval`（YAML 非空） | 启用进程内 loop；值解析同 `MaintainLoopInterval` |
| `features.disable_scheduled_maintenance` | 关闭 loop + `cmd/maintain` 间隔循环（**不**挡 **`oneclaw -maintain-once`** / **`maintain -once`**） |

## 4. 与双入口文档的关系

回合后 / 定时 **代码入口**、审计 **`post_turn_maintain` / `scheduled_maintain`**、独立 **system 模板**见 [memory-maintain-dual-entry-design.md](memory-maintain-dual-entry-design.md)。

---

*实现：`maintainloop/maintainloop.go`、`cmd/oneclaw/main.go`、`config/resolved.go`（`EmbeddedScheduledMaintainInterval`）。*
