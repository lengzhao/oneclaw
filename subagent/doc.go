// Package subagent implements nested catalog agents and run_agent (phase 4).
//
// Sub-agent catalog entries with an empty tools list use DefaultSubagentToolTemplate
// intersected with the parent registry (narrow default), not a full inheritance of parent tools.
//
// Cancellation: sub-agents use the context passed into the tool handler; when the parent
// turn is canceled, model/tool calls should abort once the runtime observes ctx.Done().
package subagent
