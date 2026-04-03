# OpenClaw 能力对照与启示

本文基于 [OpenClaw 官方文档](https://docs.openclaw.ai/) 的公开结构，对其能力形态做归纳，并说明哪些点值得 `oneclaw` 借鉴。

本文是外部参考，不代表 `oneclaw` 会逐项实现对应能力。

## 一句话关系

OpenClaw 更像完整的 Gateway 产品；`oneclaw` 更像可嵌入的 Go 运行时内核。

## OpenClaw 文档呈现出的能力域

### 1. Gateway 与运行时

- Gateway 统一会话、路由、通道与控制面。
- 提供协议层、HTTP 面、健康检查、日志、远程访问等能力。

### 2. Channels

- 覆盖多种即时通讯与协作平台。
- 强调一个 Gateway 管理多通道。

### 3. Automation

- 提供 Cron、Webhook、Hooks、Poll、Heartbeat、Background tasks 等自动化能力。

### 4. Agent / Memory / Context

- 有 Agent Runtime、Agent Loop、Workspace、Memory、Streaming、Compaction、Multi-Agent 等概念文档。

### 5. CLI 与产品操作面

- CLI 与 Web 控制面并重，说明其产品层能力较完整。

## 对 `oneclaw` 最有价值的启示

| 设计点 | 借鉴价值 | 对 `oneclaw` 的含义 |
|--------|----------|------------------|
| 单一事实源 | 高 | 明确 `host + router + bus` 是会话入队中心 |
| 概念层与运维层分离 | 高 | 维持 `concepts/`、`architecture/`、`reference/` 分层 |
| 自动化概念辨析 | 高 | 清楚区分 scheduler、heartbeat、普通用户轮次 |
| 记忆可插拔叙事 | 高 | 保持 `ConversationStore` 与长期记忆演进分离 |
| 文档入口清晰 | 高 | 维护 `docs/README.md` 作为人类与 Agent 共读入口 |
| Runbook 思维 | 中 | 把排障经验沉淀为 `reference/` 文档 |
| 重度产品控制面 | 低 | 不应反向挤压内核边界 |

## 明确不对齐的点

- 不追求 OpenClaw Gateway 协议兼容。
- 不把多通道、控制台、节点配对等产品能力设为内核默认目标。
- 不为了模仿产品形态而让 `agent.Loop` 胀成全能编排器。

## 建议的借鉴方式

推荐借鉴的是**文档组织与能力分层**，不是照搬产品边界：

1. 先把默认能力模型写清。
2. 再把架构边界写清。
3. 最后才补运维、对照和路线图。

## 相关文档

- [文档入口](../README.md)
- [默认自进化能力](../concepts/default-evolution.md)
- [范围与非目标](../concepts/scope-and-non-goals.md)
- [路线图与长期方向](./roadmap.md)
