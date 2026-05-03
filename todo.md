# oneclaw 开发待办

与 [docs/README.md](docs/README.md) 阅读顺序及 [docs/reference-architecture.md](docs/reference-architecture.md) §4 落地顺序对齐。验收条款以 [docs/requirements.md](docs/requirements.md) 为准。

---

## 阶段 0：脚手架与约定

- [x] **模块边界**：根目录包已与架构草案对齐（`config`、`paths`、`turnhub`、`engine`、`workflow`、`wfexec`、`catalog`、`preturn`、`memory`、`tools`、`adkhost`、`schedule`、`observe`、`harness`）；入站/出站形状直连 **`github.com/lengzhao/clawbridge`**。
- [x] **`go test ./...` / CI**：`go vet ./...` + `go test ./...`；[`.github/workflows/ci.yml`](.github/workflows/ci.yml)。（golangci-lint 留待后续）
- [x] **CLI 骨架**：`cmd/oneclaw` 子命令占位（`init`、`run`/`repl`、`snapshot`、`version`、`help`）；全局 `-config`、`-log-level`、`-log-format`（[`observe`](observe/log.go) + FR-CFG-04）。

---

## 阶段 1：配置 + 路径 + 最小可运行内核

目标：单配置 + 临时目录可跑通「一轮对话」类路径（可先 mock 模型），对应 PRD **FR-CFG-***、**FR-EINO-04**。

- [x] **`config`**：[`LoadMerged`](config/file.go) 深合并；[`ApplyEnvSecrets`](config/secrets.go)；[`PushRuntime` / `Runtime`](config/runtime.go)。
- [x] **`paths`**：[`ResolveUserDataRoot`](paths/paths.go)（`ONECLAW_USER_DATA_ROOT` / 配置覆盖）、`SessionRoot`、`InstructionRoot`、`Workspace`、`CatalogRoot`、[`SanitizeSessionPathSegment`](paths/sanitize.go)（`run` 写入会话目录前净化 session id）。
- [x] **`observe`**：[`AppendExecJournal`](observe/exec_journal.go) JSONL（可选细粒度执行流水；`run` 的回合级记录写在 `sessions/<id>/runs/<agent>/runs.jsonl`）。
- [x] **`adkhost`**：[`NewOpenAIChatModel`](adkhost/openai.go)、[`NewStubChatModel`](adkhost/stub.go)、[`NewChatModelAgent`](adkhost/agent.go)。
- [x] **`tools`**：[`Registry`](tools/registry.go)、[`RegisterBuiltins`](tools/builtins.go)（`echo`/`read_file`）、[`FilterByNames`](tools/registry.go)。

**烟测**：`go build -o oc ./cmd/oneclaw && oc run --mock-llm`（需已创建 workspace 等目录时由 `run` 自动 `MkdirAll`）。

---

## 阶段 2：`init` + Catalog + PreTurn + 落盘

目标：**FR-CFG-02/03**、**FR-AGT-01/04**、**FR-FLOW-03**、transcript / dialog 落盘。

- [x] **`cmd/oneclaw` `init`**：生成 `config` 模板、`AGENT.md` / MEMORY 占位、`manifest.yaml`、`agents/`、`skills/`、`workflows/` 骨架；缺键补全不覆盖用户（FR-CFG-02）。
- [x] **`catalog`**：扫描 `agents/*.md`（忽略 `README.md` / `*.readme.md`），**条目 id = 文件名（无扩展名）**；frontmatter（`name`、`tools`、`model`、`max_turns` 等）；内置 + 用户覆盖（FR-AGT-01、FR-AGT-04）。
- [x] **`preturn`**：按预算拼装 system / MEMORY / skills 摘要；Registry filter（与 FR-FLOW-03、FR-FLOW-04 衔接）。
- [x] **会话产物**：transcript / `runs/<agent_type>/` 等路径与写入（FR-AGT-05，格式与 requirements §5 一致）。

---

## 阶段 3：Workflow 规格 + 图执行 + 内置节点

目标：**FR-FLOW-02**、与 [docs/workflows-spec.md](docs/workflows-spec.md) 一致；Compose 编译为 Eino `Runnable`（[docs/eino-integration-surface.md](docs/eino-integration-surface.md) §6）。

- [x] **`workflow`**：解析 `workflow_spec_version`、`graph` / `steps`（展开为链）、根 `defaults` 浅合并进节点；DAG / 可达性 / 白名单 `use` 校验；拓扑序执行（workflows-spec §3、§7、§11 子集）。
- [x] **`wfexec`**：`ResolveWorkflowPath`（`workflows/<agent>.yaml` → manifest `workflows.default_turn`）；内置 `on_receive`、`load_prompt_md`、`load_memory_snapshot`、`filter_tools`、`adk_main`、`on_respond`、`noop`、**`agent`**；**`async: true`**：handler **goroutine** 执行，DAG 立即视为成功，[`RecordAsyncHandlerEnd`](engine/async.go) / **`AsyncHandlerFinished`**；**`use:if`** 加载期拒绝。
- [x] **`adkhost`**：`AgentOptions.Handlers`；[`observe.ChatModelLogMiddleware`](observe/adk_middleware.go)（`BeforeModelRewriteState` / `AfterModelRewriteState` + slog）。
- [x] **`engine` + Compose**：[`TurnContext`](engine/context.go)、[`RuntimeContext`](engine/runtime_context.go)；YAML **真实 DAG** 经 [`wfexec.CompilePhase3Workflow`](wfexec/compose.go)（`AllPredecessor`、YAML 边、`START`/`END`、多汇 `_oneclaw_sink`）编译；[`wfexec.Execute`](wfexec/exec.go) `Runnable.Invoke`；[`ExecMu`](engine/runtime_context.go) 串行化 handler。
- [x] **默认演进枝叶**：[`setup/templates/workflows/default.turn.yaml`](setup/templates/workflows/default.turn.yaml) 在 `on_respond` 后挂 **`memory_agent` / `skill_agent`**（`async`）；内置 Catalog [`catalog/builtin/*.md`](catalog/builtin/)（用户 `agents/` 覆盖）。**无**单独的演进加载期校验 / `TurnContext` 演进剖面；**`async`** handler 仍经 **`ExecMu`** 与同步节点串行；**`use:if`** 仍加载期拒绝。

---

## 阶段 4：子 Agent + `run_agent` + Workspace

目标：**FR-AGT-02/03/06**、[docs/appendix-data-layout.md](docs/appendix-data-layout.md) §3.1。

- [x] **`subagent` + `tools`**：`run_agent` 委托（[`subagent`](subagent/)）；子 Agent 独立 messages；工具集为父 Registry 子集（[`BuildRegistryForAgent`](subagent/registry.go)）。**`run_agent` 须在 catalog `tools` 中显式列出**；根 Agent 空 `tools` = 非 meta builtins；**子 Agent 空 `tools` = [`DefaultSubagentToolTemplate`](subagent/template.go) 与父工具求交**（不继承父全集）。
- [x] **`paths` / `preturn` / `catalog`**：`workspace: shared|private`；`inherit_parent_memory`（默认 false）；[`preturn.BuildOpts.OmitMemory`](preturn/build.go)。
- [x] **子会话目录**：[`paths.SubSessionRoot`](paths/paths.go)；[`observe.WithAgentRunAttrs`](observe/agent_run.go) + runs `detail` 含 `parent_session_id`、`sub_run_id`。

---

## 阶段 4b（backlog）：内置 Tools 扩展

阶段 1 仅交付最小 [`RegisterBuiltins`](tools/builtins.go)（`echo` / `read_file`）；本阶段扩展 builtins 与配置开关。可参考 **PicoClaw**（`config.json` 的 `tools.*` 分层、`pkg/tools` 目录能力）做**清单与配置形状**，但实现需对齐 [requirements.md](docs/requirements.md)、Workspace 边界及阶段 7 **Harness**（高风险工具默认关或包装）。

建议优先级（可按 PR 拆分）：

- [x] **FS（低风险）**：`list_dir` / `glob`；`write_file` / `append_file`；路径与 `read_file` 一致（workspace 净化、`..` 拒绝）。根注册走 [`RegisterBuiltinsForConfig`](tools/builtins.go) + [`config.File.Tools`](config/file.go)。
- [x] **编辑类**：[`edit_file`](tools/builtin/edit_file.go)（`old_text` → `new_text`，必须唯一匹配一次；对齐 PicoClaw `edit_file`）。
- [x] **配置开关**：根配置 `tools.<name>.enabled`（[`config/tools.go`](config/tools.go)）；`catalog` `tools:` 仍为 Agent 级 allowlist。
- [x] **子 Agent 模板**：[`DefaultSubagentToolTemplate`](subagent/template.go) 仍为读向默认（`echo` / `read_file` / `list_dir`）；`glob` / 写类需父 registry 与 catalog 显式下放。
- [x] **`exec`**：[`tools/builtin/exec.go`](tools/builtin/exec.go) + [`config.ExecCommandPermitted`](config/tools.go)；**默认关闭**（显式 `tools.exec.enabled: true` + `allow` 前缀白名单，`deny` 子串）；运行时策略读 [`config.Runtime`](config/runtime.go)。浏览器 / Web / MCP 挪到阶段 7。

---

## 阶段 5：TurnHub + 渠道 + 定时

目标：统一入站形状、出站 Bus、[docs/reference-architecture.md](docs/reference-architecture.md) §2.3、§2.6。

- [x] **`turnhub`**：`SessionHandle`、mailbox、`serial` / `insert`（及文档约定的抢占策略若需要）。
- [x] **入站/出站**：`InboundMessage` / Bus / `Reply` 等直接使用 **clawbridge**（含 **webchat**）；`serve` 消费总线 → TurnHub → `runner`，定时任务同路径。
- [x] **`schedule`**：持久化 jobs、poller、合成 `InboundMessage` 走与用户需求相同主路径。

---

## 阶段 6：记忆演进 + Skills（异步与 staging）

目标：MEMORY/skills 流水线（workflow + 内置 / 可覆盖 agents）、[docs/eino-md-chain-architecture.md](docs/eino-md-chain-architecture.md) §3、[docs/appendix-data-layout.md](docs/appendix-data-layout.md) §6；演进闭环校验若需要单列 backlog。

- [ ] **`memory`**：staging / `write_behavior_policy`、晋升；与 **`workflows/*.yaml`** 中 **`memory_agent` async 枝**对接。
- [ ] **`wfexec`**：主 ADK 之后链后继节点（记忆抽取、Skills 生成）；异步默认、可配置「reply 前 flush」。
- [ ] **`catalog` / `workflow`**：演进类 `agent_type` 与 workflow 绑定及审计字段。

---

## 阶段 7（可选）：RAG + Harness + 浏览器 / Web / MCP

- [ ] **RAG**：按 FR-KNOW-* 与 eino-ext 装配 Embedder / Indexer / Retriever；知识库路径默认 `knowledge/sources/`（附录 §6）。
- [ ] **`harness`**：SafeHarness、高风险工具包装（[docs/harness-governance-extensions.md](docs/harness-governance-extensions.md)）；与 **`exec` 配额 / 审计** 可增强衔接。
- [ ] **浏览器 / Web fetch**：默认关闭或强约束；与 harness、配额、审计设计后再接线。
- [ ] **MCP**：发现、搜索、桥接；与 [FR-FLOW-04](docs/requirements.md) 衔接。

---

## 快速对照：包 → 主交付

| 包 | 主交付 |
|----|--------|
| `config` | 合并配置、env、默认 profile |
| `paths` | 数据根与会话布局 |
| `catalog` | `agents/*.md` Catalog |
| `workflow` / `wfexec` | YAML DAG → Eino Graph |
| `adkhost` / `tools` | ADK + Registry + run_agent；内置扩展见 **阶段 4b** |
| `preturn` / `memory` | 注入与记忆策略 |
| `engine` / `turnhub` | 回合生命周期与排队 |
| clawbridge（依赖）/ `schedule` | 多通道与定时 |
| `observe` | 日志与 execution 记录 |
| `harness` | 治理扩展 |

完成某一阶段后，在 PR 或提交说明中标注对应的 **FR-*** 编号便于追溯。
