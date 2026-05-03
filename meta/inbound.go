// Package meta holds stable cross-package constants (avoids import cycles between runner, schedule, tools).
package meta

// Inbound metadata keys on clawbridge InboundMessage.Metadata (host → runner).
const (
	InboundAgent       = "oneclaw.agent"
	InboundProfile     = "oneclaw.profile"
	InboundMockLLM     = "oneclaw.mock_llm"
	InboundCorrelation = "oneclaw.correlation_id"
	InboundScheduleJob = "schedule.id"
)

// Synthetic schedule inbound (schedule package → clawbridge).
const (
	SourceKey      = "oneclaw.source"
	SourceSchedule = "schedule"
)
