package runner

import "github.com/lengzhao/oneclaw/meta"

// Metadata keys on clawbridge InboundMessage.Metadata used when the host maps an inbound turn to [Params].
const (
	InboundMetaAgent       = meta.InboundAgent
	InboundMetaProfile     = meta.InboundProfile
	InboundMetaMockLLM     = meta.InboundMockLLM
	InboundMetaCorrelation = meta.InboundCorrelation
	InboundMetaScheduleJob = meta.InboundScheduleJob
)
