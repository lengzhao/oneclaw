# oneclaw 开发待办

与 [docs/README.md](docs/README.md) 阅读顺序及 [docs/reference-architecture.md](docs/reference-architecture.md) §4 落地顺序对齐。验收条款以 [docs/requirements.md](docs/requirements.md) 为准。

---

## 阶段 0：脚手架与约定

- [x] **模块边界**：根目录包已与架构草案对齐（`config`、`paths`、`channel`、`turnhub`、`engine`、`workflow`、`wfexec`、`catalog`、`preturn`、`memory`、`tools`、`adkhost`、`schedule`、`observe`、`harness`）。
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

- [ ] **`tools`**：`run_agent`（或等价）委托；子 Agent 独立 messages；工具集为父 Registry 子集。
- [ ] **`paths` / `preturn`**：`workspace: shared|private`；`inherit_parent_memory` 等显式开关（默认不继承主 MEMORY）。
- [ ] **子会话目录**：`sessions/<parent>/subs/<sub_run>/` 或等价策略，日志带 `parent_session_id`、`sub_run_id`。

---

## 阶段 5：TurnHub + 渠道 + 定时

目标：统一入站形状、出站 Bus、[docs/reference-architecture.md](docs/reference-architecture.md) §2.3、§2.6。

- [ ] **`turnhub`**：`SessionHandle`、mailbox、`serial` / `insert`（及文档约定的抢占策略若需要）。
- [ ] **`channel`**：`InboundMessage` / `publishOutbound`；对接 **clawbridge**（可选）与 CLI/HTTP 最小驱动。
- [ ] **`schedule`**：持久化 jobs、poller、合成 `InboundMessage` 走与用户需求相同主路径。

---

## 阶段 6：记忆演进 + Skills（异步与 staging）

目标：MEMORY/skills 流水线（workflow + 内置 / 可覆盖 agents）、[docs/eino-md-chain-architecture.md](docs/eino-md-chain-architecture.md) §3、[docs/appendix-data-layout.md](docs/appendix-data-layout.md) §6；演进闭环校验若需要单列 backlog。

- [ ] **`memory`**：staging / `write_behavior_policy`、晋升；与 **`workflows/*.yaml`** 中 **`memory_agent` async 枝**对接。
- [ ] **`wfexec`**：主 ADK 之后链后继节点（记忆抽取、Skills 生成）；异步默认、可配置「reply 前 flush」。
- [ ] **`catalog` / `workflow`**：演进类 `agent_type` 与 workflow 绑定及审计字段。

---

## 阶段 7（可选）：RAG + Harness

- [ ] **RAG**：按 FR-KNOW-* 与 eino-ext 装配 Embedder / Indexer / Retriever；知识库路径默认 `knowledge/sources/`（附录 §6）。
- [ ] **`harness`**：SafeHarness、高风险工具包装（[docs/harness-governance-extensions.md](docs/harness-governance-extensions.md)）。

---

## 快速对照：包 → 主交付

| 包 | 主交付 |
|----|--------|
| `config` | 合并配置、env、默认 profile |
| `paths` | 数据根与会话布局 |
| `catalog` | `agents/*.md` Catalog |
| `workflow` / `wfexec` | YAML DAG → Eino Graph |
| `adkhost` / `tools` | ADK + Registry + run_agent |
| `preturn` / `memory` | 注入与记忆策略 |
| `engine` / `turnhub` | 回合生命周期与排队 |
| `channel` / `schedule` | 多通道与定时 |
| `observe` | 日志与 execution 记录 |
| `harness` | 治理扩展 |

完成某一阶段后，在 PR 或提交说明中标注对应的 **FR-*** 编号便于追溯。
