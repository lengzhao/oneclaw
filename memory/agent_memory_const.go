package memory

// ScheduledMaintainIsolationSessionID is the github.com/lengzhao/memory isolation session id for
// RunScheduledMaintain extracts. SelectRecall merges hits from this scope with the live chat session
// so consolidated memories stored under this key remain visible during recall.
const ScheduledMaintainIsolationSessionID = "scheduled_maintain"
