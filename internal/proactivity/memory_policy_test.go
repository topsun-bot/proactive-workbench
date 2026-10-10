package proactivity_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/topsun-bot/proactive-workbench/internal/clock"
	"github.com/topsun-bot/proactive-workbench/internal/datasources/calendar"
	"github.com/topsun-bot/proactive-workbench/internal/datasources/situation"
	"github.com/topsun-bot/proactive-workbench/internal/datasources/weather"
	"github.com/topsun-bot/proactive-workbench/internal/memory"
	"github.com/topsun-bot/proactive-workbench/internal/proactivity"
)

func TestLikesOutdoorsFalseSuppressesPark(t *testing.T) {
	s := sensors(
		weather.FixtureClear,
		situation.MustMock(situation.PlaceHome, situation.ActivityIdle),
		calendar.NewMock(),
		saturdayAfternoon(),
	)
	snap := memory.Empty()
	if err := snap.UpsertPreference(memory.Preference{Key: memory.PrefLikesOutdoors, Value: "false"}); err != nil {
		t.Fatal(err)
	}
	res, err := proactivity.TickWithMemory(context.Background(), s, proactivity.DefaultPolicy(), proactivity.NewDedupe(), snap)
	if err != nil {
		t.Fatal(err)
	}
	if res.Goal.Kind == proactivity.GoalVisitPark && res.Decision.Interrupt {
		t.Fatal("likes_outdoors=false must not interrupt with park")
	}
	if !strings.Contains(res.Goal.Reason, "likes_outdoors") && res.Goal.Kind == proactivity.GoalVisitPark {
		t.Fatalf("reason %q", res.Goal.Reason)
	}
}

func TestMemoryQuietHoursOverridePolicy(t *testing.T) {
	s := sensors(
		weather.FixtureClear,
		situation.MustMock(situation.PlaceHome, situation.ActivityIdle),
		calendar.NewMock(),
		saturdayAfternoon(),
	)
	snap := memory.Empty()
	_ = snap.UpsertPreference(memory.Preference{Key: memory.PrefQuietStart, Value: "15"})
	_ = snap.UpsertPreference(memory.Preference{Key: memory.PrefQuietEnd, Value: "16"})
	res, err := proactivity.TickWithMemory(context.Background(), s, proactivity.DefaultPolicy(), proactivity.NewDedupe(), snap)
	if err != nil {
		t.Fatal(err)
	}
	if res.Decision.Interrupt {
		t.Fatal("15:00 is quiet hours from memory")
	}
	if !strings.Contains(res.Decision.Reason, "quiet hours") {
		t.Fatalf("reason %q", res.Decision.Reason)
	}
}

func TestCommitmentNudgeWhenParkUnavailable(t *testing.T) {
	loc := shanghai()
	clk := clock.Fixed(time.Date(2026, 10, 10, 15, 0, 0, 0, loc), loc)
	s := sensors(
		weather.FixtureClear,
		situation.MustMock(situation.PlaceWork, situation.ActivityIdle),
		calendar.NewMock(),
		clk,
	)
	snap := memory.Empty()
	if err := snap.UpsertCommitment(memory.Commitment{
		ID:     "rev",
		Title:  "FIXTURE weekly review with FIXTURE colleague",
		When:   time.Date(2026, 10, 10, 16, 30, 0, 0, loc),
		Notes:  "",
		Source: memory.FixtureSource,
	}); err != nil {
		t.Fatal(err)
	}
	if err := snap.UpsertPerson(memory.Person{
		ID: "fixture-colleague", Name: "FIXTURE colleague", Relation: "coworker", Source: memory.FixtureSource,
	}); err != nil {
		t.Fatal(err)
	}
	res, err := proactivity.TickWithMemory(context.Background(), s, proactivity.DefaultPolicy(), proactivity.NewDedupe(), snap)
	if err != nil {
		t.Fatal(err)
	}
	if res.Goal.Kind != proactivity.GoalCommitmentNudge {
		t.Fatalf("goal %s reason %s", res.Goal.Kind, res.Goal.Reason)
	}
	if !res.Decision.Interrupt {
		t.Fatalf("commitment should interrupt: %s", res.Decision.Reason)
	}
	if !strings.Contains(res.Goal.Reason, "FIXTURE colleague") {
		t.Fatalf("should mention remembered person: %s", res.Goal.Reason)
	}

	dedupe := proactivity.NewDedupe()
	first, err := proactivity.TickWithMemory(context.Background(), s, proactivity.DefaultPolicy(), dedupe, snap)
	if err != nil {
		t.Fatal(err)
	}
	again, err := proactivity.TickWithMemory(context.Background(), s, proactivity.DefaultPolicy(), dedupe, snap)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Decision.Interrupt || again.Decision.Interrupt {
		t.Fatalf("dedupe failed first=%v again=%v", first.Decision, again.Decision)
	}
}
