# Init 布局说明

`oneclaw init` 将嵌入的 **`setup/templates/`** 整树按相对路径拷到 **UserDataRoot**（已存在文件 **不覆盖**；`config.yaml` 已存在则只 **合并缺失键** — **FR-CFG-02**）。维护默认布局：往 **`setup/templates/`** 增删文件即可（见 **`setup/embed.go`** 的 **`//go:embed templates`**）。

## 用户目录里有什么

| 路径 | 说明 |
|------|------|
| `config.yaml` | 模型、工具、`clawbridge` 等 |
| `manifest.yaml` | `default_agent`、`workflows.default_turn` |
| `AGENT.md` / `MEMORY.md` | 会话说明与记忆占位 |
| `workflows/*.yaml` | 主回合与子 Agent（`memory_extractor` / `skill_generator`）workflow |
| `agents/default.md` | 默认 Catalog Agent（**Go `text/template`** 渲染 `UserDataRoot`） |
| `agents/README.md` | 若模板中存在，会一并拷入（Catalog **不**把 `README.md` 当 Agent 加载） |
| **`skills/**`** | 含默认 **`skill-creator/`**（见下）与 **`skills/README.md`** |
| `prompts/` | 预留 prompt override / prompt assets |
| `knowledge/sources/` | 预留知识库原文入口 |
| `sessions/` | 会话根目录 |

## **`skill-creator`（默认随 init 下发）**

**`skill-creator`** 定义如何把**可重复的问题模式**写成 **`skills/<id>/`** 树（`SKILL.md` + 可选 `scripts/`、`reference/`）。它与内置 **`skill_generator`**、默认 workflow 里异步 **`skill_agent`** 一起，承担 **「从对话与 run journal 里提炼 → 落盘为 skill → 后续回合再注入」** 的 **能力固化** 闭环，是 oneclaw 的**核心产品路径之一**，不是边缘示例。

仓库 **`examples/skills/skill-creator/`** 与嵌入模板 **`setup/templates/skills/skill-creator/`** 内容应对齐，便于不跑 `init` 时在 Git 里直接浏览。

## `agents/` 目录

在 **`agents/`** 下放 `*.md`（YAML frontmatter + 正文）。Catalog **id** = 文件名去掉扩展名。`README.md`、`*.readme.md`、`*.tmpl` 不参与加载。

高级：可在 Catalog 根下增加 **`agents/<agent_type>.prompt.tmpl`** 覆盖主回合布局，见 `docs/eino-md-chain-architecture.md`。
