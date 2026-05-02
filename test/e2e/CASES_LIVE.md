# E2E 用例清单（真实 LLM / Live）

与 **[CASES.md](./CASES.md)**（`openaistub`、默认 CI）分工如下：

| 方式 | 负责范围 |
|------|----------|
| **Stub** | 异常路径、边界条件、鉴权失败、空输入、工具拒绝、确定性编排等——**在 Stub 侧覆盖**。 |
| **真实 LLM** | **行为与落盘是否在真实模型下符合预期**：说明文件 / 任务是否按设计更新、与**设计预期**相比有何**差异**。 |

「自学 / 进化」在本项目中的含义见根目录 [`README.md`](../../README.md) 与 [`docs/self-evolution-plan.md`](../../docs/self-evolution-plan.md)：**不是微调权重**，而是 **说明文件与任务落盘** + **规则可写回**（Live 关心 **LLM 生成质量与后续轮次行为**）。

**不默认进 CI**（费用、时延、非确定性）。

---

## 1. 环境与门禁

| 项 | 说明 |
|----|------|
| **客户端** | **不要**把 `openai.base_url` 指到 `openaistub`；使用官方或兼容网关（见 `live_llm.config.yaml`）。 |
| **鉴权** | 合并 YAML 中的 `openai.api_key`（见 [`docs/config.md`](../../docs/config.md)）。 |
| **模型** | YAML `model`；**同一套 Live 场景建议固定模型与版本**，便于对比「预期 vs 实际」的时间序列。 |
| **传输** | 主路径为 **Eino**；网关差异主要通过 **`openai.base_url`** 与服务商行为验收。 |
| **隔离** | `t.TempDir()` 为 cwd；`HOME` 与 YAML `paths.memory_base`（测试中可用 `e2eIsolateUserMemory`）指向临时目录，避免污染本机。 |
| **门禁** | `RUN_LIVE_LLM=1` + `testing.Short()` 跳过；避免 CI 无密钥仍跑 Live。 |

---

## 2. 验收核心：预期、实际与差异

Live 的价值是发现 **Stub 测不到的模型侧问题**（忽略约束、摘要丢关键信息等）。建议按下面结构记录（可写在 PR 或独立 `live-runs.md` 中）：

| 字段 | 内容 |
|------|------|
| **设计预期** | 在本轮输入与配置下，**磁盘 / 下一 prompt** 应出现什么（例如：转写更新、`MEMORY.md` 经工具写回、语义 compact 边界标记等）。 |
| **实际观测** | **文件摘录**、**下一轮行为**、**token/轮数**。 |
| **与预期的差异** | 无 / 有：模型方差 vs 产品 bug。 |
| **元数据** | 日期、`model`、网关、合并 YAML / `PushRuntime` 中与 `budget`、`features.*` 相关的关键项。 |

自动化时：**只对「管道是否触发、文件是否生成」做强断言**；对 **自然语言质量** 以人工 + 差异记录为主。

---

## 3. 用例总表（与 Stub 对照）

| LIVE-E | 摘要 | Stub 对照 | 验收焦点 |
|--------|------|-----------|----------|
| **LIVE-E1** | **语义 compact**：撑满预算触发 compact 后，任务仍满足早期约束。 | E2E-103、E2E-104 | 摘要是否丢条件。 |
| **LIVE-E2** | **指令注入**：`AGENT.md` / `MEMORY.md` 在真模型下是否被遵守。 | E2E-10～E2E-14 | 模型是否遵循显式规则。 |
| **LIVE-E3** | **（可选）写回规则**：`write_behavior_policy` 等工具写回后下一轮是否生效。 | 随功能补 Stub | 写回可解析、可执行。 |

**请仍用 Stub 覆盖**：无 Key、空输入、ctx cancel、越权路径、未知工具名、损坏 transcript 等（见 [CASES.md](./CASES.md)）。

---

## 4. 推荐执行顺序

1. **LIVE-E2**：指令与规则。  
2. **LIVE-E1**：长上下文与 compact。  
3. **LIVE-E3**：随写回类产品能力交付再开。

---

## 5. 运行方式（建议）

- 默认 **`go test ./test/e2e/...` 仍全部 Stub**。  
- Live 建议 `//go:build live_llm` 或：

```go
if os.Getenv("RUN_LIVE_LLM") != "1" {
    t.Skip("evolution live: set RUN_LIVE_LLM=1")
}
if testing.Short() {
    t.Skip("short")
}
```

- 每条 **LIVE-E** 跑完填写 §2 差异表，便于模型升级后做回归对比。

---

## 6. 交叉引用

- 管道步骤、文件布局、Enqueue 顺序仍以 **[CASES.md](./CASES.md)** 为详规。  
- 本文档只定义 **真实 LLM 下「进化是否生效」** 的验收维度与 **预期/差异** 记录方式。
