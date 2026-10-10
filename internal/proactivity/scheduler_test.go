package proactivity_test

import (
	"testing"
	"time"

	"github.com/topsun-bot/proactive-workbench/internal/clock"
	"github.com/topsun-bot/proactive-workbench/internal/memory"
	"github.com/topsun-bot/proactive-workbench/internal/proactivity"
)

func TestMorningBriefDueAtConfiguredTime(t *testing.T) {
	loc := shanghai()
	before := clock.Fixed(time.Date(2026, 10, 10, 7, 59, 0, 0, loc), loc)
	at := clock.Fixed(time.Date(2026, 10, 10, 8, 0, 0, 0, loc), loc)
	snap := memory.FixtureSnapshot()

	early := proactivity.NewScheduler(before, time.Minute)
	if due := kinds(early.Due(snap)); containsKind(due, proactivity.RoutineMorningBrief) {
		t.Fatal("07:59 must not fire morning_brief at 08:00")
	}

	onTime := proactivity.NewScheduler(at, time.Minute)
	if due := kinds(onTime.Due(snap)); !containsKind(due, proactivity.RoutineMorningBrief) {
		t.Fatalf("08:00 should fire morning_brief, got %v", due)
	}
	onTime.MarkRan(proactivity.RoutineMorningBrief, time.Date(2026, 10, 10, 8, 0, 0, 0, loc))
	if due := kinds(onTime.Due(snap)); containsKind(due, proactivity.RoutineMorningBrief) {
		t.Fatal("already ran today")
	}
}

func TestSenseTickInterval(t *testing.T) {
	loc := shanghai()
	start := time.Date(2026, 10, 10, 15, 0, 0, 0, loc)
	clk := clock.Fixed(start, loc)
	s := proactivity.NewScheduler(clk, 15*time.Minute)
	snap := memory.Empty()
	if due := kinds(s.Due(snap)); !containsKind(due, proactivity.RoutineSenseTick) {
		t.Fatal("first sense_tick is due")
	}
	s.MarkRan(proactivity.RoutineSenseTick, start)
	if due := kinds(s.Due(snap)); containsKind(due, proactivity.RoutineSenseTick) {
		t.Fatal("just ran")
	}

	later := proactivity.NewScheduler(clock.Fixed(start.Add(15*time.Minute), loc), 15*time.Minute)
	later.MarkRan(proactivity.RoutineSenseTick, start)
	if due := kinds(later.Due(snap)); !containsKind(due, proactivity.RoutineSenseTick) {
		t.Fatal("15m later should be due")
	}
}

func kinds(rs []proactivity.Routine) []proactivity.RoutineKind {
	out := make([]proactivity.RoutineKind, 0, len(rs))
	for _, r := range rs {
		out = append(out, r.Kind)
	}
	return out
}

func containsKind(ks []proactivity.RoutineKind, want proactivity.RoutineKind) bool {
	for _, k := range ks {
		if k == want {
			return true
		}
	}
	return false
}
