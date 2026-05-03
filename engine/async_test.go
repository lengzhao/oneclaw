package engine

import "testing"

func TestRuntimeContext_AsyncHandlerFinished(t *testing.T) {
	rtx := &RuntimeContext{}
	rtx.RecordAsyncHandlerEnd("n", nil)
	done, err := rtx.AsyncHandlerFinished("n")
	if !done || err != nil {
		t.Fatalf("done=%v err=%v", done, err)
	}
	done, err = rtx.AsyncHandlerFinished("missing")
	if done {
		t.Fatal("expected not done")
	}
}
