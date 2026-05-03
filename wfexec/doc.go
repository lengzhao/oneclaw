// Package wfexec resolves workflow paths, registers built-in "use" handlers, and runs workflows by
// compiling YAML graphs to Eino compose.Graph with WithNodeTriggerMode(AllPredecessor) (DAG semantics).
// Multi-sink graphs connect sinks through an internal _oneclaw_sink merge node so END has one edge.
// Nodes with yaml async: true run their handler in a new goroutine; the DAG treats the step as
// succeeded immediately so dependents run without waiting. Completion is recorded via
// engine.RuntimeContext.RecordAsyncHandlerEnd / AsyncHandlerFinished (success => nil error).
// Sync and async handlers share RuntimeContext.ExecMu — avoid long critical sections.
// Async hand-off freezes read-mostly fields via engine.WithReadSnapshot on the handler context only
// (Effective* reads ReadSnapshotFromContext(GoCtx)).
package wfexec
