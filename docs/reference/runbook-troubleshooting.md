# oneclaw 运维速查与排错

面向自建宿主，适用于嵌入 `host` + `bus` + `agent.Loop` 的服务，或直接运行 `cmd/oneclaw`。

架构背景见 [ADR-001：模块边界与接口形态](../architecture/adr-001-module-boundaries.md)。

## 快速检查清单

| 检查项 | 建议 |
|--------|------|
| 日志里是否有 `trace_id` | 用同一 `trace_id` 串起一次用户轮次与总线处理 |
| `Publish` 返回值 | `nil` 表示入队成功；`ErrBackpressure` 表示该会话队列已满 |
| HTTP Webhook 返回 503 | 多为背压或关停中 |
| 客户端断开 | 若 `InboundEvent.Ctx` 绑定请求 context，取消会传到 `Loop.Run` |

## 背压：`bus.ErrBackpressure`

**现象**：`Host.Publish` / `Bus.Publish` 返回 `bus: session queue full`。

**原因**：该 `SessionID` 对应的有界缓冲已满，消费者处理速度跟不上；设计为非阻塞，避免无限堆积内存。

**可选应对**：

1. 调大队列：创建 `bus.NewBus` 时增大 `queueSize`。
2. 降载：生产者侧重试、丢弃非关键事件，或对 HTTP 返回 `503/429` 让上游退避。
3. 加快消费：缩短单次 `Loop` 耗时，限制工具次数或 `MaxIterations`。

## 进程关停与 `Bus.Close`

**现象**：关停后继续 `Publish` 可能 panic。

**原因**：`Bus.Close` 会关闭各会话 channel；之后任何 `Publish` 都不安全。

**建议**：仅在进程退出路径调用 `Close`；关闭前先停止接收新请求，并等待队列中的事件尽量处理完。

## Webhook 与 `scheduler`

**现象**：返回 `queue busy` 或 HTTP 503。

**原因**：可能来自背压，也可能来自其他 publish 失败。

**生产建议**：

- 校验调用方身份，例如 Token、HMAC、IP allowlist。
- 限制 body 大小与 JSON 深度。
- 对同一 `session_id` 做客户端限流，减少单会话队列堆积。

## Context 与取消

`InboundEvent.Ctx` 非空时，总线使用该 context 调用 handler，进而执行 `agent.Loop.Run(ctx, agent.RunInputFromInbound(&e))`。

这意味着：

- HTTP 请求断开时，正在执行的 `Loop` 可能被取消。
- `LLM` 与 `Tools` 实现应尊重 `ctx` 并及时返回。
- Cron 等无请求上下文的场景通常使用 `context.Background()`，行为与 HTTP 不同。

## Skills 与 workspace

| 现象 | 说明 |
|------|------|
| 日志 `skills: unknown name, skipping` | `metadata["skills"]` 或默认列表里含有未注册名字；检查拼写与目录文件名 |
| 工作区未生效 | 确认 `-workspace` 指向目录，且存在 `IDENTITY.md`、`SOUL.md`、`AGENTS.md`、`USER.md` |

## 出站回调 `host.OnOutbound`

若设置了 `Host.OnOutbound`，其返回的 `error` 会使总线 handler 失败，并记录 `bus handler failed`。用于回写通道或二次 Webhook 时，应在回调内部自行控制超时与重试。

## 仍无法定位时

1. 提高 `slog` 日志级别。
2. 用同一 `session` 与 `trace_id` 过滤日志。
3. 对照数据流：`Publish` → `router` → `bus` → `Loop`。

如果排错结果应固化为默认行为，优先补充：

- `reference/` 文档：当它属于操作方法
- `concepts/` 文档：当它影响默认能力认知
- ADR：当它改变架构边界
