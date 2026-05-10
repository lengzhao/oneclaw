package schedule

import (
	"context"
	"testing"
	"time"
)

func TestStartDailyJobAtLocalHourInvalid(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := StartDailyJobAtLocalHour(ctx, -1, func() {}); err == nil {
		t.Fatal("expected error for hour -1")
	}
	if err := StartDailyJobAtLocalHour(ctx, 24, func() {}); err == nil {
		t.Fatal("expected error for hour 24")
	}
}

func TestStartDailyJobAtLocalHourStartsAndStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	n := 0
	if err := StartDailyJobAtLocalHour(ctx, 12, func() { n++ }); err != nil {
		t.Fatal(err)
	}
	cancel()
	time.Sleep(50 * time.Millisecond)
	if n != 0 {
		t.Fatalf("unexpected fn runs before scheduled fire: n=%d", n)
	}
}
