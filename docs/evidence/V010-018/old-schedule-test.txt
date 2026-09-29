package audit

import (
	"context"
	"testing"
	"time"
)

func TestScheduledMaintenanceAutomaticallyExpiresNewDueData(t *testing.T) {
	l, c := clean(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	runs := 0
	err := Scheduled(ctx, l, c, 10*time.Millisecond, func() time.Time { return now }, func(r Result, err error) {
		if err != nil {
			t.Errorf("scheduled maintenance failed")
		}
		runs++
		if runs == 1 {
			event(t, c, "archive.authentication_events", eventID, "2025-09-01T00:00:00Z", "newly-due")
		}
		if runs == 2 {
			if count(t, c, "archive.authentication_events") != 0 {
				t.Error("next scheduled cycle must automatically delete due data")
			}
			cancel()
		}
	})
	if runs != 2 || err != nil {
		t.Fatal("automatic maintenance must execute immediate and subsequent cycle, then stop on cancellation")
	}
}
