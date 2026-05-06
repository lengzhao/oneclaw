package structuredmem

import (
	lzmem "github.com/lengzhao/memory"
	lzmodel "github.com/lengzhao/memory/model"
)

// StructuredMemoryExtractRequest builds the default lzmem.ExtractRequest for main-turn
// structured memory extraction (matches lzmem prompt-default-v3 + transient ExtractPolicy).
// Callers may set ReferenceTime, TimeZone, ResolutionContext, PostExtractHook (host/plugin), or override ExtractPolicy.
func StructuredMemoryExtractRequest(dialog string, llm *lzmodel.LLMConfig) lzmem.ExtractRequest {
	return lzmem.ExtractRequest{
		DialogText:    dialog,
		LLMConfig:     llm,
		MinConfidence: 0.7,
		DryRun:        false,
		ExtractPolicy: &lzmem.ExtractPolicy{DropTransientEphemeral: true},
	}
}
