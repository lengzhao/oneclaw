# 用户指南（最短路径）

面向「先跑起来再读架构」的使用说明；深度设计见 [README.md](README.md) 索引。

**默认产品路径**：**自选 LLM** + **可选 clawbridge 渠道**，数据落在 **`~/.oneclaw`**；向导会写入 **`key_files/*.json`** 并合并 **`models`** / **`default_model`**；LLM 与渠道都支持 **skip**（保留默认或已有配置）。交互终端上渠道一步与 LLM 类似——**按序号选择已注册的 driver**（与 **`oneclaw channel list-drivers`** 一致），**0** 跳过；可多次添加；背景方案仍可参考 [极简 onboard](plans/weixin-alibaba-oauth-minimal-onboarding.md)。

---

## 快速启动（默认 `~/.oneclaw`：微信 + 自选 LLM）

### 0）安装二进制

在仓库根目录任选其一：

- **`go install ./cmd/oneclaw`** → 命令通常在 **`$GOBIN` 或 `$GOPATH/bin`**；
- **`go build -o oneclaw ./cmd/oneclaw`** → 使用 **`./oneclaw …`**（当前目录这份）。

请确认 **`oneclaw version`**：第二行应为 **`module github.com/lengzhao/oneclaw`**。若 **`init`/`onboard`** 却出现 **`webchat listening`** 且不退出，多半是 PATH 指向了**别的/旧版**同名程序 —— 请 **`which oneclaw`** 或改用 **`./oneclaw`**。

### 1）三步上手（不写 `--user-data` 即默认 **`~/.oneclaw`**）

| 步骤 | 命令 | 做什么 |
|------|------|--------|
| 1 | **`oneclaw init`** | 生成 **`~/.oneclaw`** 下模板：`config.yaml`、`agents/`、`workflows/`、`key_files/`（0700）等，见 [examples/init/README.md](../examples/init/README.md)。 |
| 2 | **`oneclaw onboard`** | **LLM（可选）**：菜单选厂商（OpenAI / Claude / Gemini / Ark / Moonshot / Qwen(DashScope) / DeepSeek / OpenRouter / 自定义网关），**`0` 跳过**（保留默认或已有模型配置）；选择厂商后打开**控制台**粘贴 **API Key** → **`key_files/*.json`**，再填 **模型名**（Ark 为先填 **Endpoint ID**），并合并 **`models`** + **`default_model`**。**渠道（可选）**：序号菜单选择 **`clawbridge` onboarding driver**（如 **weixin**、**webchat** 等，取决于已注册实现）；**`0` 跳过**（保留已有客户端配置）；同一向导内可「继续添加其他渠道」。其中 **webchat manual** 会自动写入 **`enabled: true`** 与默认 **`listen/path/display_name`**，无需手改。等价单步命令：**`oneclaw channel onboard <driver>`**。 |
| 3 | **`oneclaw serve`** | 拉起 clawbridge + TurnHub；在微信里发消息即可对话（模板里 webchat 默认关，**不靠 webchat 做主路径**）。 |

完成后可用 **`oneclaw config show`** 看头部 **`# user_data_root`**（应为 **`~/.oneclaw`**）、脱敏后的 **`models`** / **`clawbridge`**。

#### 与 OpenClaw「直接登录」的差别

OpenClaw 的向导可按供应商走浏览器 **OAuth**、CLI 复用等；凭证集中在 **`auth-profiles.json`** 一类存储。**oneclaw** 仅支持 **API Key**：向导写入 **`key_files/*.json`**（或你使用 **`api_key_env`**），**不进行 OAuth 与令牌刷新**（规格见 [unified-model-auth](plans/unified-model-auth.md)）。

**向导可选参数**：**`--skip-llm`**、**`--skip-drivers`**、**`--mock-llm`**、**`--no-browser`**、**`--listen ADDR`**（仅当选中的渠道为 **weixin** 时传给 **`channel onboard`**，浏览器扫码页兜底）。stdin 非终端时：LLM 菜单默认走 skip，渠道步骤直接跳过（需要时再执行 **`oneclaw channel onboard <driver>`**）。详见 **`oneclaw onboard -h`**。

---

## 可选：离线 mock / 自定义数据目录

- **不接微信、不接真模型**：**`oneclaw init`** → **`oneclaw run --mock-llm`**（可加 **`-prompt "一句话"`**；别名 **`oneclaw repl`**）。
- **自定义目录**：**`export ONECLAW_USER_DATA_ROOT=/你的目录`** 后 **`oneclaw init`**；后续 **`run` / `serve` / `config show`** 须在**同一环境变量**下执行（或在该目录 **`config.yaml`** 写 **`user_data_root`**）。若仅用 **`init --user-data /path`** 不设环境变量，默认命令仍会找 **`~/.oneclaw`**，易踩坑。

---

## 1. 安装与目录

- **`ONECLAW_USER_DATA_ROOT`**：覆盖默认 **`~/.oneclaw`**（与 **`config.user_data_root`** 优先级见 **`paths.ResolveUserDataRoot`**）。
- **`oneclaw init`**：**`--user-data DIR`** 指定目录；省略则 **`~/.oneclaw`** 或环境变量。

---

## 2. DashScope（阿里云 Qwen）

在 **`oneclaw onboard`** 中选 **Qwen（DashScope）**：向导写入 **`key_files/dashscope.json`**（**`api_key`**），并在 **`config.yaml`** 里合并 **`provider: qwen`**、**`auth.token_file`** 与 **`default_model`**。不使用 RAM OAuth。

---

## 3. `config show` 与单机 `run`（进阶）

- **`oneclaw config show`**：合并后的有效配置（默认注入 env + **`key_files`** 再脱敏）；**`--raw`** 只看 YAML 合并、不注入密钥。
- **`oneclaw run`**：单次 CLI 回合（不经微信）；模型选择器为 **`profile_id_or_provider/api模型`**（仅第一个 **`/`**）。真模型需 **`OPENAI_API_KEY`**（模板 **`default`** profile）或 **`key_files` + `auth.token_file`**（如 onboard 后的阿里云）。

---

## 4. 常见失败

| 现象 | 处理 |
|------|------|
| **`init`/`onboard` 只有 webchat listening、不退出** | PATH 指错二进制；**`which oneclaw`**、**`./oneclaw`** 或 **`go install ./cmd/oneclaw`**（仓库根）。 |
| **`run` 报找不到 workflow/agent**，目录不像刚 init | **`ONECLAW_USER_DATA_ROOT`** / **`config.user_data_root`** 是否与 **`config show` 头部**一致（参见上文「可选：自定义目录」）。 |
| `serve` 报没有 enabled client | 重新 **`oneclaw onboard`** 完成微信；或仅本机调试时在 **`config.yaml`** 把 **webchat** 设为 **`enabled: true`**。 |
| 模型 401 / missing API key | **`config show`**；核对 **`models[].auth.token_file`** 指向的 **`key_files/*.json`** 是否含 **`api_key`**，或模板里的 **`api_key_env`** 是否已 **`export`**。 |
| 微信扫码无反应 | **`oneclaw channel onboard weixin -listen 127.0.0.1:端口`** 用浏览器页兜底。 |

---

## 5. 追溯

- **极简微信 + 阿里云 onboard（对标 OpenClaw 收窄路径）**：[plans/weixin-alibaba-oauth-minimal-onboarding.md](plans/weixin-alibaba-oauth-minimal-onboarding.md)  
- **多厂商统一鉴权（规格）**：[plans/unified-model-auth.md](plans/unified-model-auth.md)  
- PRD：[requirements.md](requirements.md)
