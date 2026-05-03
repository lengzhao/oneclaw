package schedule

import "sync"

var wakeMu sync.Mutex
var wakeListeners []chan struct{}

// SubscribeWake registers for [notifyScheduleWake] (e.g. after jobs file changes). Call cancel when done.
func SubscribeWake() (notify <-chan struct{}, cancel func()) {
	ch := make(chan struct{}, 1)
	wakeMu.Lock()
	wakeListeners = append(wakeListeners, ch)
	wakeMu.Unlock()
	return ch, func() {
		wakeMu.Lock()
		defer wakeMu.Unlock()
		for i, c := range wakeListeners {
			if c == ch {
				wakeListeners = append(wakeListeners[:i], wakeListeners[i+1:]...)
				break
			}
		}
	}
}

func notifyScheduleWake() {
	wakeMu.Lock()
	defer wakeMu.Unlock()
	for _, ch := range wakeListeners {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
