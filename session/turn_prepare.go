package session

import (
	"context"
	"log/slog"
	"os"

	"github.com/lengzhao/oneclaw/budget"
	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/routing"
	"github.com/lengzhao/oneclaw/rtopts"
	"github.com/lengzhao/oneclaw/subagent"
	"github.com/lengzhao/oneclaw/toolctx"
)

// sharedTurnPrep is the common substrate for SubmitUser and submitLocalSlashTurn:
// tool context, memory bundle, system prompt string, subagent runner, and optional outbound emitter.
type sharedTurnPrep struct {
	tctx    *toolctx.Context
	bg      budget.Global
	memOK   bool
	layout  memory.Layout
	bundle  memory.TurnBundle
	em      *routing.Emitter
	system  string
	catalog *subagent.Catalog
}

// prepareSharedTurn builds tctx, memory recall/agent blocks, ResolveTurnSink + Emitter, buildTurnSystem, and subRunner.
// wireSendMessage sets tctx.SendMessage when true (full model turns only).
func (e *Engine) prepareSharedTurn(ctx context.Context, in routing.Inbound, preview string, wireSendMessage bool) (sharedTurnPrep, error) {
	var p sharedTurnPrep
	p.bg = rtopts.Current().Budget
	p.tctx = toolctx.New(e.CWD, ctx)
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
	sink, err := routing.ResolveTurnSink(ctx, e.SinkRegistry, e.SinkFactory, in)
	if err != nil {
		return sharedTurnPrep{}, err
	}
	if sink != nil {
		p.em = routing.NewEmitter(sink, e.SessionID, "")
	}
	cat := subagent.LoadCatalog(e.CWD)
	p.catalog = cat
	p.system = e.buildTurnSystem(p.memOK, p.bundle, p.bg, home, herr, cat)
	p.tctx.Subagent = &subRunner{eng: e, turnSystem: p.system, catalog: cat, bg: p.bg}
	return p, nil
}
