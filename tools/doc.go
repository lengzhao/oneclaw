// Package tools registers executable capabilities for ADK ChatModel agents.
//
// # Required vs extended builtins (roadmap)
//
// **P0 — minimal kernel (current)**  
//   - `read_file`: read UTF-8 text under [Registry.WorkspaceRoot] (required for workspace-bound agents).  
//   - `echo`: smoke/tests only — not in [DefaultBuiltinIDs]; register explicitly or list in agent `tools:` when needed.
//
// **P1 — filesystem (partial)**  
//   - `list_dir` / `glob`: enumerate / match paths under workspace (caps + path rules align with `read_file`).  
//   - `write_file` / `append_file` / `edit_file`: mutate workspace (`edit_file`: exact single occurrence replace).
//
// **Meta (not in RegisterBuiltins)**  
//   - `run_agent`: registered by [github.com/lengzhao/oneclaw/subagent] when explicitly allowed.
//
// **P2+ — harness / policy gated**  
//   - `exec`: configure under tools.exec (default off); browser / web / MCP align with docs/requirements.md and phase 7 harness.
//
// # Layout (builtins)
//
// Implementations and name constants live in [github.com/lengzhao/oneclaw/tools/builtin] (one package, multiple files, main-branch style).
// [RegisterBuiltinsNamed] wires tools explicitly via a switch — add a case and an Infer* factory when introducing a builtin.
// Workspace path rules: [github.com/lengzhao/oneclaw/tools/workspace].
//
// # Eino interfaces (avoid parallel abstractions)
//
// Use only CloudWeGo Eino tool types — no duplicate tool execution interfaces in this package:
//
//   - Create callables with [github.com/cloudwego/eino/components/tool/utils.InferTool] (or InferEnhancedTool when returning multimodal [schema.ToolResult]).
//   - Optional: [github.com/cloudwego/eino/components/tool/utils.NewTool] when schema is built manually.
//   - Store everything as tool.BaseTool; runnable tools implement tool.InvokableTool (or Enhanced/Stream variants), assignable to BaseTool without adapters.
//   - [Registry] is a thin name-ordered wrapper; [toolhost.Registry] exposes the subset needed for sub-agent filtering.
//
// ADK receives the same []BaseTool from [Registry.All]; no conversion layer beyond interface satisfaction.
package tools
