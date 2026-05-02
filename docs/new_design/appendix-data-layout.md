# 附录：用户数据根与 Workspace 布局（摘要）

返回：[README.md](README.md) · [requirements.md](requirements.md) §5

本文为 **`requirements.md` §5** 与 **`reference-from-oneclaw.md`** 中路径概念的**自洽展开**；复制到新项目后**无需**依赖 oneclaw 其他 `docs/` 文件。细节演进以实现为准。

---

## 1. 术语（与 `glossary.md` 对齐）

- **UserDataRoot**：用户级根路径（示例：`~/.oneclaw`）。
- **InstructionRoot**：**`AGENT.md` 与 `MEMORY.md` 必须同一目录**下的「说明与规则记忆入口」。
- **Workspace**：默认作为文件工具 / `exec` 等工作目录的路径；oneclaw 中通常为 `<InstructionRoot>/workspace`（目录名固定为 `workspace`）。
- **SessionRoot**：`UserDataRoot/sessions/<session_id>/`，承载该会话的转写等；是否兼作 InstructionRoot 由 **会话隔离** 策略决定。

---

## 2. 未开启会话隔离（扁平）

- **InstructionRoot = UserDataRoot**。
- 典型包含：`config.yaml`、`AGENT.md`、`MEMORY.md`、`rules/`、`workspace/`、`sessions/<id>/transcript*.json`、`scheduled_jobs.json`（位置以实现为准）。

---

## 3. 开启会话隔离

- **InstructionRoot = SessionRoot**（该会话的「小用户根」），其下仍应有配对的 `AGENT.md` / `MEMORY.md` 与同构的 `workspace/`。
- 全局 `UserDataRoot` 仍可保留全局 `config.yaml` 与可选全局基线说明文件。

---

## 4. 设计意图（便于新项目取舍）

1. **配置与「干活目录」分离**：降低误删配置/记忆入口的风险。
2. **同会话内路径一致**：隔离模式下会话内相对布局与扁平模式「形状一致」，仅根路径从 UserDataRoot 换成 SessionRoot。
3. **数据根树内避免嵌套同名隐藏目录**：实现上常以固定子目录名（如 `workspace/`、`sessions/`）表达，而非多层 `.app/.app`。

---

## 5. 与 FR 的对应

| 需求文档中的概念 | 本附录位置 |
|------------------|------------|
| `sessions.isolate_workspace`、共享 vs 每会话 workspace | §2–§3 |
| IM 转写路径 under `sessions/<id>/` | §2 |
