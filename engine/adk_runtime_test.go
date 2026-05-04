package engine

import "testing"

func TestADKRuntime_setters_nilSafe(t *testing.T) {
	var rtx *RuntimeContext
	rtx.SetAssistant("x")
	rtx.SetSawOnRespond(true)
	rtx.SetChatAgent(nil)
}

func TestADKRuntime_setters(t *testing.T) {
	var rtx RuntimeContext
	rtx.SetAssistant("hello")
	rtx.SetSawOnRespond(true)
	if rtx.Assistant != "hello" || !rtx.SawOnRespond {
		t.Fatalf("%q %v", rtx.Assistant, rtx.SawOnRespond)
	}
}
