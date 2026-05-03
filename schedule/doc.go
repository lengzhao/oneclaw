// Package schedule persists jobs under UserDataRoot/scheduled_jobs.json and ticks a poller that emits synthetic inbound turns.
//
// Execution path (with serve): due jobs → synthetic clawbridge InboundMessage → same TurnHub + runner path as live channels (reference-architecture section 2.6).
// Cron jobs: after each fire, the next [Job.NextRunUnix] is at least [MinSecondsBetweenJobFires] after the fire time, even if the cron expression would fire sooner.
package schedule
