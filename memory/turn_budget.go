package memory

import "github.com/lengzhao/oneclaw/budget"

// ApplyTurnBudget shrinks TurnBundle fields to respect Global caps (recall should already match RecallBytes).
func ApplyTurnBudget(b *TurnBundle, g budget.Global) {
	if b == nil || !g.Enabled() {
		return
	}
	sysCap, agentCap := g.InjectCaps()
	if len(b.SystemSuffix) > sysCap {
		b.SystemSuffix = budget.TruncateUTF8(b.SystemSuffix, sysCap)
	}
	if len(b.AgentMdBlock) > agentCap {
		b.AgentMdBlock = budget.TruncateUTF8(b.AgentMdBlock, agentCap)
	}
	recallCap := g.RecallBytes()
	if len(b.RecallBlock) > recallCap {
		b.RecallBlock = budget.TruncateUTF8(b.RecallBlock, recallCap)
	}
}
