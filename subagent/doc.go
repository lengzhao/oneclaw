// Package subagent runs nested user turns (run_agent, fork_context) via [Host.RunTurn]
// (oneclaw routes this to the same Eino TurnRunner as the parent session).
//
// Tool surface: run_agent uses FilterRegistry against the catalog allowlist, then
// WithoutMetaTools to drop run_agent and fork_context; fork_context only applies the
// meta strip when the nesting rules require it. See docs/inbound-routing-design.md §9.
package subagent
