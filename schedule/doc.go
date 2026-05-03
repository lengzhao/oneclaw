// Package schedule persists jobs under UserDataRoot/scheduled_jobs.json and runs a poller that emits synthetic inbound turns.
//
// Execution path (with serve): due jobs → synthetic clawbridge InboundMessage → same TurnHub + runner path as live channels (reference-architecture section 2.6).
// Delivery loop ([RunPollerLoop]): sleep until earliest [Job.NextRunUnix] (≥ [MinTimerSleep], aligned with [MinGranularitySeconds]), or wake immediately when the cron tool updates the store ([SubscribeWake]); idle sleep [IdleTimerSleep] when no jobs.
// Cron jobs: after each fire, the next [Job.NextRunUnix] is at least [MinSecondsBetweenJobFires] after the fire time, even if the cron expression would fire sooner.
// Cron tool add: [mergeScheduleKinds] requires at_seconds, every_seconds, and RFC3339 lead time to respect [MinGranularitySeconds].
package schedule
