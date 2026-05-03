package engine

type asyncHandlerSlot struct {
	done bool
	err  error
}

// RecordAsyncHandlerEnd records completion of a fire-and-forget (workflow async) node handler.
// handlerErr is nil on success. Safe to call once per nodeID per run.
func (rtx *RuntimeContext) RecordAsyncHandlerEnd(nodeID string, handlerErr error) {
	if rtx == nil || nodeID == "" {
		return
	}
	rtx.asyncMu.Lock()
	defer rtx.asyncMu.Unlock()
	if rtx.asyncSlots == nil {
		rtx.asyncSlots = make(map[string]*asyncHandlerSlot)
	}
	rtx.asyncSlots[nodeID] = &asyncHandlerSlot{done: true, err: handlerErr}
}

// AsyncHandlerFinished reports whether an async node's handler has finished.
// When finished is true, err is nil on success or the handler error on failure.
func (rtx *RuntimeContext) AsyncHandlerFinished(nodeID string) (finished bool, handlerErr error) {
	if rtx == nil || nodeID == "" {
		return false, nil
	}
	rtx.asyncMu.Lock()
	defer rtx.asyncMu.Unlock()
	if rtx.asyncSlots == nil {
		return false, nil
	}
	s := rtx.asyncSlots[nodeID]
	if s == nil || !s.done {
		return false, nil
	}
	return true, s.err
}
