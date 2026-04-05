package channel

import (
	"context"

	"github.com/lengzhao/oneclaw/routing"
	"github.com/lengzhao/oneclaw/session"
)

func submitLoop(ctx context.Context, in <-chan InboundTurn, eng *session.Engine, source string) {
	for {
		select {
		case <-ctx.Done():
			return
		case turn, ok := <-in:
			if !ok {
				return
			}
			submitOne(ctx, turn, eng, source)
		}
	}
}

func submitOne(parentCtx context.Context, turn InboundTurn, eng *session.Engine, source string) {
	tctx := turn.Ctx
	if tctx == nil {
		tctx = parentCtx
	}
	sk := turn.SessionKey
	if sk == "" {
		sk = eng.SessionID
	}
	inb := routing.Inbound{
		Source:        source,
		Text:          turn.Text,
		SessionKey:    sk,
		UserID:        turn.UserID,
		TenantID:      turn.TenantID,
		CorrelationID: turn.CorrelationID,
	}
	err := eng.SubmitUser(tctx, inb)
	if turn.Done != nil {
		select {
		case turn.Done <- err:
		default:
		}
	}
}
