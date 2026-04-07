# 多 LLM / 多协议支持 — 技术调整说明

本文描述在 **不恢复环境变量配置**（仍以合并后的 YAML 为唯一配置源，见 [config.md](config.md)）的前提下，让 oneclaw 像 [picoclaw](../picoclaw/pkg/providers/factory_provider.go) 一样支持多种 LLM 后端所需的**架构调整**与**分阶段落地**。

## 1. 现状（oneclaw）

| 层次 | 现状 |
|------|------|
| 配置 | 顶层 `model` + `openai.api_key` / `openai.base_url` / `org_id` / `project_id`；维护单独 `maintain.model`、`maintain.scheduled_model` |
| 客户端 | 全局一个 `openai.Client`，由 `config.Resolved.OpenAIOptions()` 构造 |
| 会话与循环 | `session.Engine` 持有 `openai.Client`；`loop.Config` 含 `*openai.Client` + `Model` 字符串 |
| 补全调用 | `model.Complete` / `CompleteWithTransport` 直接调 `client.Chat.Completions`（流式 / 非流式） |
| 消息与工具 | 全链路 `openai.ChatCompletionMessageParamUnion`、`openai.ChatCompletionToolParam` |
| 其它入口 | `memory/maintain_run`、`subagent/run` 等同样依赖 `*openai.Client` 与同型消息 |

**结论**：能力上已支持任意 **OpenAI Chat Completions 兼容** 网关（通过 `base_url`）；**未**按「协议前缀」拆分多后端，也**未**内置 Anthropic Messages 等非 OpenAI 形态 API。

## 2. 目标能力（对齐 picoclaw 思路）

1. **显式协议**：模型配置采用 `协议/模型ID`（与 picoclaw `ExtractProtocol` 一致）；无前缀时约定默认协议（建议默认 `openai`，与现网 `model: gpt-4o` 兼容）。
2. **多后端**：在统一抽象下挂载多类实现，例如：
   - **OpenAI 兼容 HTTP**（当前能力 + 各厂商默认 `base_url` 表，可选）
   - **Anthropic**（API Key HTTP 或原生 Messages 包，视实现阶段而定）
   - **Azure / Bedrock**（独立 endpoint 与鉴权，后续迭代）
3. **凭据仍走 YAML**（及未来若有的本地凭据存储）：**不**把 `OPENAI_API_KEY` 等环境变量作为运行时配置源（与现有产品决策一致）。
4. **可选**：主模型 + fallback 链（错误分类后切换），与 picoclaw `FallbackChain` 同思路；可作为 Phase 2+。

## 3. 推荐架构

### 3.1 对外配置形态（草案）

在保持向后兼容的前提下，二选一或组合：

**方案 A（最小改动）**  
- 保留 `model: openai/gpt-4o` 或 `model: gpt-4o`（默认协议 `openai`）。  
- 保留全局 `openai:` 块作为 **默认连接参数**（api_key、base_url）。  
- 若某协议需要独立 key/base，再增加 **按名称的模型条目**（见方案 B）。

**方案 B（与 picoclaw 控制台模型列表接近）**  

```yaml
# 全局默认连接（可省略，由下面 models 条目覆盖）
openai:
  api_key: ""
  base_url: ""

# 可选：命名模型列表；agents / 路由可引用 model_ref
models:
  chat:
    model: "openai/gpt-4o"           # 或 groq/llama-3.3-70b-versatile
    api_key: ""                      # 空则回退 openai.api_key
    base_url: ""                     # 空则使用该协议内置默认或全局 openai.base_url
  cheap:
    model: "deepseek/deepseek-chat"
    api_key: "..."
    base_url: "https://api.deepseek.com/v1"

model: chat                          # 指向 models 的键，或直接写 model 字符串
```

合并规则、路径、`PushRuntime` 的约定延续 [config.md](config.md)；新增字段需在 `config.File` / `Resolved` 上提供解析与校验。

### 3.2 内部抽象：`llm` 包（建议新建）

定义与 OpenAI SDK 解耦的**窄接口**，供 `loop.RunTurn` 调用：

```text
Provider interface {
  Chat(ctx, req ChatRequest) (*ChatResponse, error)
  // 可选：ChatStream，与 model 包 transport 策略对齐
}
```

- **ChatRequest**：system、messages、tools、model（**已去掉协议前缀的 model ID** 或完整 ref，由工厂约定）、max_tokens、transport 提示等。  
- **ChatResponse**：规范化 assistant 文本、tool_calls、usage；由 `loop` 再转回当前存储用的 `openai.ChatCompletionMessageParamUnion`（**Phase 1 推荐**：减少 JSONL / transcript 迁移面），或中长期改为中性 `Message` 类型并做一次序列化迁移。

**默认实现 `OpenAICompatProvider`**：内部持有 `*openai.Client`（或按 base_url 构造的 client），调用现有 `model.CompleteWithTransport`，把 `ChatCompletion` 映射为 `ChatResponse`。

**工厂 `NewProviderForResolved(…)`**：  
- 解析 `protocol/modelID`（与 picoclaw `ExtractProtocol` / `NormalizeProvider` 行为对齐，可抽小模块或拷贝归一化表）。  
- `switch protocol`：`openai`、`groq`、`openrouter`、… → `OpenAICompatProvider` + 默认 base_url；`anthropic` → 专用 provider（Phase 2）。

### 3.3 `loop` 改造要点

- `loop.Config`：将 `Client *openai.Client` 替换为 `LLM llm.Provider`（或同时保留 Client 仅用于过渡期适配器）。  
- `RunTurn` 内构建 `ChatRequest`，调用 `LLM.Chat`；工具执行仍用 `tools.Registry.OpenAITools()`，在 provider 边界转换为请求格式（OpenAI 兼容实现可为直通）。  
- `usageledger`：继续依赖 response 中的 token 字段；若某 provider 不返回 usage，需定义占位或跳过策略。

### 3.4 `session.Engine` 与入口

- 由 `cmd` / 装配层根据 `Resolved` 构造 `llm.Provider`，注入 `Engine`（或只注入 `loop.Config`）。  
- `Engine` 可保留 `Model` 字符串（完整 `protocol/model` 或解析后的 id，需在文档与代码中统一一种）。

### 3.5 维护与子 Agent

- `memory/maintain_run`、`subagent`：将 `*openai.Client` 改为接受同一 `llm.Provider`（或工厂按 `maintain.model` 再建一个 provider）。  
- `maintain.model` 支持 `协议/模型`；解析规则与主会话一致。

## 4. 分阶段落地（建议）

| 阶段 | 内容 | 风险 |
|------|------|------|
| **Phase 0** | 文档与配置草案定稿；`model` 允许写 `openai/xxx`（解析后仍用单一全局 Client，仅改传入 API 的 model 字符串） | 低 |
| **Phase 1** | 引入 `llm.Provider` + `OpenAICompatProvider`；`loop` / `session` 走接口；多协议前缀共用一个 Client，按协议选默认 `base_url`、按条目选 key | 中（测试面大） |
| **Phase 2** | Anthropic（或 Messages）provider；工具/消息在边界做映射 | 高 |
| **Phase 3** | Fallback 链、Azure/Bedrock、per-channel 模型覆盖 | 中 |

## 5. 测试与验收

- 单元：`ExtractProtocol`、默认 base_url、`Resolved` 合并模型条目。  
- 集成：现有 `openaistub` 继续验证 OpenAI 路径；为第二协议增加 stub 或录制测试。  
- E2E：`test/e2e` 中 engine 构造改为可注入 `llm.Provider`。

## 6. 非目标（本文不展开）

- 在 oneclaw 内复制 picoclaw 的 **OAuth 设备码 / 浏览器登录** 全流程（可另立「凭据子系统」文档）。  
- 与 picoclaw 仓库 **源码级 vendor** 整包合并；仅借鉴协议表与工厂模式即可。

## 7. 参考实现位置（picoclaw 树内）

- 协议表与工厂：`picoclaw/pkg/providers/factory_provider.go`（`protocolMetaByName`、`CreateProviderFromConfig`）。  
- 模型引用解析：`picoclaw/pkg/providers/model_ref.go`。  
- OpenAI 兼容 HTTP：`picoclaw/pkg/providers/http_provider.go` → `openai_compat`。  
- Fallback：`picoclaw/pkg/providers/fallback.go`。

---

**文档维护**：实现过程中若 YAML 形状或包名与本文不一致，以 `config/project_init.example.yaml` 与 `docs/config.md` 为准，并回写本节「对外配置形态」。
