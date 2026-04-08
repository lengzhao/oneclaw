package session

import (
	"context"
	"log/slog"
	"os"

	"github.com/lengzhao/clawbridge/bus"
	"github.com/lengzhao/oneclaw/budget"
	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/rtopts"
	"github.com/lengzhao/oneclaw/subagent"
	"github.com/lengzhao/oneclaw/toolctx"
)

// sharedTurnPrep is the common substrate for SubmitUser and submitLocalSlashTurn:
// tool context, memory bundle, system prompt string, subagent runner, and optional outbound hook.
type sharedTurnPrep struct {
	tctx         *toolctx.Context
	bg           budget.Global
	memOK        bool
	layout       memory.Layout
	bundle       memory.TurnBundle
	outboundText func(ctx context.Context, text string) error
	system       string
	catalog      *subagent.Catalog
	turnSnap     bus.InboundMessage
}

// prepareSharedTurn builds tctx, memory recall/agent blocks, optional OutboundText, buildTurnSystem, and subRunner.
// wireSendMessage sets tctx.SendMessage when true (full model turns only).
// parentTurnID and parentCorrelationID are used for subagent notify correlation (same values as the current user turn).
func (e *Engine) prepareSharedTurn(ctx context.Context, in bus.InboundMessage, atts []Attachment, preview string, wireSendMessage bool, parentTurnID, parentCorrelationID string) (sharedTurnPrep, error) {
	var p sharedTurnPrep
	p.turnSnap = in
	p.bg = rtopts.Current().Budget
	p.tctx = toolctx.New(e.CWD, ctx)
	p.tctx.AgentID = e.EffectiveRootAgentID()
	if wireSendMessage {
		p.tctx.SendMessage = e.SendMessage
	}
	home, herr := os.UserHomeDir()
	if herr == nil {
		p.tctx.HomeDir = home
	}
	p.memOK = herr == nil && !rtopts.Current().DisableMemory
	if p.memOK {
		p.layout = memory.DefaultLayout(e.CWD, home)
		p.bundle = memory.BuildTurn(p.layout, home, preview, &e.RecallState, p.bg.RecallBytes())
		memory.ApplyTurnBudget(&p.bundle, p.bg)
		if p.bundle.UpdatedRecall != nil {
			e.RecallState = *p.bundle.UpdatedRecall
		}
		p.tctx.MemoryWriteRoots = p.layout.WriteRoots()
	} else if herr != nil {
		slog.Warn("session.user_home", "err", herr)
	}
	if e.PublishOutbound != nil {
		snap := in
		p.outboundText = func(ctx context.Context, text string) error {
			msg := assistantTextOutbound(&snap, text)
			if msg == nil {
				return nil
			}
			return e.PublishOutbound(ctx, msg)
		}
	}
	cat := subagent.LoadCatalog(e.CWD)
	p.catalog = cat
	p.system = e.buildTurnSystem(p.memOK, p.bundle, p.bg, home, herr, cat)
	p.tctx.Subagent = &subRunner{
		eng:                 e,
		turnSystem:          p.system,
		catalog:             cat,
		bg:                  p.bg,
		parentTurnID:        parentTurnID,
		parentCorrelationID: parentCorrelationID,
	}
	return p, nil
}
