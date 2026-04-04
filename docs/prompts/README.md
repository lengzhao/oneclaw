# Claude Code Prompt 文档索引

这个目录把原来的 `prompt-templates/` 和 `prompt-examples/` 合并成一套文档。

合并后的原则很简单：

- 每个主题只保留一页
- 每页同时包含：
  - 结构模板
  - 半实化示例
  - 关键观察点
- 不再让“抽象结构”和“真实观感”分散在两个目录里来回跳

---

## 文件说明

- `00-request-envelope.md`
  一次完整模型请求的总外壳，顺带说明 `system`、`messages`、`tools`、attachments 的最终位置。

- `10-main-thread.md`
  主线程 prompt 的结构和典型观感。

- `20-subagent.md`
  普通子 Agent 的 prompt 结构、上下文裁剪和角色化差异。

- `30-fork-agent.md`
  fork agent 的 cache-safe 前缀复用方式。

- `40-teammate.md`
  teammate / swarm 的协作 prompt 结构。

- `50-memory.md`
  memory 相关块的模板、示例和位置区分。

---

## 推荐阅读顺序

1. `00-request-envelope.md`
2. `10-main-thread.md`
3. `20-subagent.md`
4. `30-fork-agent.md`
5. `40-teammate.md`
6. `50-memory.md`

---

## 这套合并版解决什么问题

之前两套目录的分工是清楚的：

- `prompt-templates/` 强调结构
- `prompt-examples/` 强调观感

但实际阅读时会有两个问题：

1. 同一个主题要在两个目录间来回切
2. 结构说明和例子容易重复维护

现在改成单页后，每个主题都可以按同一顺序看：

1. 先看抽象结构
2. 再看半实化示例
3. 最后看这一页的结论

---

## 关键源码入口

- 主线程 system prompt：`src/constants/prompts.ts`
- user/system context 注入：`src/utils/api.ts`
- 主线程入口：`src/QueryEngine.ts`
- query 主循环：`src/query.ts`
- 子 Agent prompt：`src/tools/AgentTool/runAgent.ts`
- fork agent：`src/utils/forkedAgent.ts`
- teammate addendum：`src/utils/swarm/teammatePromptAddendum.ts`
- memory prompt：`src/memdir/memdir.ts`
