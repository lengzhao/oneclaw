package runner

import "github.com/lengzhao/oneclaw/meta"

// Metadata keys on clawbridge InboundMessage.Metadata used when the host maps an inbound turn to [Params].
const (
	InboundMetaAgent = meta.InboundAgent
	// InboundMetaProfile selects profile_id_or_provider/api_model_id (same as oneclaw run --profile).
	InboundMetaProfile     = meta.InboundProfile
	InboundMetaMockLLM     = meta.InboundMockLLM
	InboundMetaCorrelation = meta.InboundCorrelation
	InboundMetaScheduleJob = meta.InboundScheduleJob
)
