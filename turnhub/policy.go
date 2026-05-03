package turnhub

// Policy selects how new inbound items interact with the per-session mailbox (reference-architecture section 2.2).
type Policy uint8

const (
	// PolicySerial queues inbound turns FIFO (default for mixed user + schedule traffic).
	PolicySerial Policy = iota
	// PolicyInsert drops any pending turns and keeps only the latest inbound (burst coalescing).
	PolicyInsert
	// PolicyPreempt currently matches PolicyInsert for the pending queue; cancelling an in-flight turn is not implemented.
	PolicyPreempt
)
