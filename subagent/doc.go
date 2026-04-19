// Package subagent runs nested loop.RunTurn sessions (run_agent, fork_context).
//
// Tool surface: run_agent uses FilterRegistry against the catalog allowlist, then
// WithoutMetaTools to drop run_agent and fork_context; fork_context only applies the
// meta strip when the nesting rules require it. See docs/inbound-routing-design.md §9.
package subagent
