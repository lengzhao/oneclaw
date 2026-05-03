// Package turnhub multiplexes inbound turns per SessionHandle with serial/insert/preempt mailbox policies.
//
// Host invariant (oneclaw serve): every user-visible turn—including clawbridge drivers (e.g. webchat) and schedule synthetic messages—
// should enter [Hub.Enqueue] before [runner.ExecuteTurn]; do not call [runner.ExecuteTurn] directly for channel traffic so session ordering stays consistent.
package turnhub
