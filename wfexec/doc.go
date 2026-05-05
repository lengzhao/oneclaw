// Package wfexec resolves workflow paths, registers built-in "use" handlers, and runs workflows by
// compiling YAML workflow v2 specs to Eino compose.Workflow.
// Nodes with yaml async: true run their handler in a new goroutine; the workflow treats the step as
// succeeded immediately so dependents run without waiting. Completion is recorded via
// engine.RuntimeContext.RecordAsyncHandlerEnd / AsyncHandlerFinished (success => nil error).
// Sync and async handlers share RuntimeContext.ExecMu — avoid long critical sections.
// Async hand-off freezes read-mostly fields via engine.WithReadSnapshot on the handler context only
// (Effective* reads ReadSnapshotFromContext(GoCtx)).
package wfexec
