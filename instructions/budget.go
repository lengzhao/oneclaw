package instructions

import "github.com/lengzhao/oneclaw/budget"

// ApplyTurnBudget shrinks TurnBundle fields to respect Global caps.
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
}
