package personnel

import (
	"encoding/json"
	"testing"
	"time"
)

func TestQ36FrozenCalendarBoundsAreReusedExactly(t *testing.T) {
	raw := json.RawMessage(`{"operator":"and","children":[{"field":"occurredAt","operator":"eq","value":{"date":"2026-03-08","timeZone":"America/New_York"}}]}`)
	plan, err := CompileFilter("events", raw, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.FrozenDates) != 1 || plan.FrozenDates[0].From.Format(time.RFC3339) != "2026-03-08T05:00:00Z" || plan.FrozenDates[0].To.Sub(plan.FrozenDates[0].From) != 23*time.Hour {
		t.Fatalf("first query must retain actual DST bounds: %+v", plan.FrozenDates)
	}
	// A stored boundary is authoritative even if a later timezone rule database
	// would resolve the public calendar expression differently.
	retained := []QueryRange{{From: time.Date(2026, 3, 8, 4, 0, 0, 0, time.UTC), To: time.Date(2026, 3, 9, 4, 0, 0, 0, time.UTC)}}
	next, err := compileFrozenFilter("events", raw, 1, retained)
	if err != nil || len(next.Arguments) != 2 || next.Arguments[0] != retained[0].From || next.Arguments[1] != retained[0].To {
		t.Fatalf("stored boundaries recomputed: %+v %v", next, err)
	}
	for _, bounds := range [][]QueryRange{{}, {{From: retained[0].To, To: retained[0].From}}, {retained[0], retained[0]}} {
		if _, err := compileFrozenFilter("events", raw, 1, bounds); err == nil {
			t.Fatal("mismatched/corrupt frozen bounds accepted")
		}
	}
}
