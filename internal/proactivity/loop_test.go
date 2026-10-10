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
	"github.com/topsun-bot/proactive-workbench/internal/proactivity"
)

func shanghai() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		panic(err)
	}
	return loc
}

func saturdayAfternoon() clock.Clock {
	loc := shanghai()
	now := time.Date(2026, 10, 10, 15, 0, 0, 0, loc)
	return clock.Fixed(now, loc)
}

func fridayAfternoon() clock.Clock {
	loc := shanghai()
	now := time.Date(2026, 10, 9, 13, 0, 0, 0, loc)
	return clock.Fixed(now, loc)
}

func sensors(wx string, sit *situation.Mock, cal calendar.Source, clk clock.Clock) proactivity.Sensors {
	return proactivity.Sensors{
		Weather:   weather.MustMock(wx),
		Calendar:  cal,
		Situation: sit,
		Clock:     clk,
		Location:  weather.DefaultLocation,
	}
}

func TestParkGoalInterruptsThenDedupe(t *testing.T) {
	s := sensors(
		weather.FixtureClear,
		situation.MustMock(situation.PlaceHome, situation.ActivityIdle),
		calendar.NewMock(),
		saturdayAfternoon(),
	)
	mem := proactivity.NewMemory()
	first, err := proactivity.Tick(context.Background(), s, proactivity.DefaultPolicy(), mem)
	if err != nil {
		t.Fatal(err)
	}
	if first.Goal.Kind != proactivity.GoalVisitPark {
		t.Fatalf("goal %s", first.Goal.Kind)
	}
	if !first.Decision.Interrupt {
		t.Fatalf("first tick should interrupt: %s", first.Decision.Reason)
	}

	second, err := proactivity.Tick(context.Background(), s, proactivity.DefaultPolicy(), mem)
	if err != nil {
		t.Fatal(err)
	}
	if second.Goal.Kind != proactivity.GoalVisitPark {
		t.Fatalf("goal still park, got %s", second.Goal.Kind)
	}
	if second.Decision.Interrupt {
		t.Fatal("second tick must not interrupt when nothing changed")
	}
	if !strings.Contains(second.Decision.Reason, "unchanged") {
		t.Fatalf("reason %q", second.Decision.Reason)
	}
	if first.Decision.Fingerprint != second.Decision.Fingerprint {
		t.Fatal("fingerprint must be stable across identical ticks")
	}
}

func TestQuietHoursBlocksInterrupt(t *testing.T) {
	loc := shanghai()
	clk := clock.Fixed(time.Date(2026, 10, 10, 23, 30, 0, 0, loc), loc)
	s := sensors(
		weather.FixtureClear,
		situation.MustMock(situation.PlaceHome, situation.ActivityIdle),
		calendar.NewMock(),
		clk,
	)
	res, err := proactivity.Tick(context.Background(), s, proactivity.DefaultPolicy(), proactivity.NewMemory())
	if err != nil {
		t.Fatal(err)
	}
	if res.Decision.Interrupt {
		t.Fatal("23:30 is quiet hours")
	}
	if !strings.Contains(res.Decision.Reason, "quiet hours") {
		t.Fatalf("reason %q", res.Decision.Reason)
	}
}

func TestRainTomorrowProducesUmbrellaGoal(t *testing.T) {
	s := sensors(
		weather.FixtureRain,
		situation.MustMock(situation.PlaceHome, situation.ActivityIdle),
		calendar.NewMock(),
		fridayAfternoon(),
	)
	res, err := proactivity.Tick(context.Background(), s, proactivity.DefaultPolicy(), proactivity.NewMemory())
	if err != nil {
		t.Fatal(err)
	}
	if res.Goal.Kind != proactivity.GoalUmbrellaReminder {
		t.Fatalf("goal %s reason %s", res.Goal.Kind, res.Goal.Reason)
	}
	if !res.Decision.Interrupt {
		t.Fatalf("umbrella reminder should interrupt: %s score %d", res.Decision.Reason, res.Goal.Score)
	}
	if !strings.Contains(strings.Join(res.Plan.Steps, " "), "umbrella") {
		t.Fatalf("plan should mention umbrella flow: %#v", res.Plan.Steps)
	}
}

func TestExistingUmbrellaEventSuppressesGoal(t *testing.T) {
	cal := calendar.NewMock()
	cal.Add(calendar.Event{
		UID:   "already-there",
		Title: "Bring an umbrella / 带伞",
		Start: time.Date(2026, 10, 10, 8, 0, 0, 0, shanghai()),
		End:   time.Date(2026, 10, 10, 8, 15, 0, 0, shanghai()),
	})
	s := sensors(
		weather.FixtureRain,
		situation.MustMock(situation.PlaceHome, situation.ActivityIdle),
		cal,
		fridayAfternoon(),
	)
	res, err := proactivity.Tick(context.Background(), s, proactivity.DefaultPolicy(), proactivity.NewMemory())
	if err != nil {
		t.Fatal(err)
	}
	if res.Decision.Interrupt {
		t.Fatalf("should not interrupt when reminder already exists: %#v", res)
	}
}

func TestMeetingSoonBlocksPark(t *testing.T) {
	cal := calendar.NewMock()
	cal.Add(calendar.Event{
		UID:   "meet-1",
		Title: "FIXTURE: 1:1",
		Start: time.Date(2026, 10, 10, 16, 0, 0, 0, shanghai()),
		End:   time.Date(2026, 10, 10, 16, 30, 0, 0, shanghai()),
	})
	s := sensors(
		weather.FixtureClear,
		situation.MustMock(situation.PlaceHome, situation.ActivityIdle),
		cal,
		saturdayAfternoon(),
	)
	res, err := proactivity.Tick(context.Background(), s, proactivity.DefaultPolicy(), proactivity.NewMemory())
	if err != nil {
		t.Fatal(err)
	}
	if res.Decision.Interrupt {
		t.Fatalf("meeting in 1h must block park interrupt: score %d reason %s", res.Goal.Score, res.Decision.Reason)
	}
}

func TestAtWorkNoPark(t *testing.T) {
	s := sensors(
		weather.FixtureClear,
		situation.MustMock(situation.PlaceWork, situation.ActivityIdle),
		calendar.NewMock(),
		saturdayAfternoon(),
	)
	res, err := proactivity.Tick(context.Background(), s, proactivity.DefaultPolicy(), proactivity.NewMemory())
	if err != nil {
		t.Fatal(err)
	}
	if res.Goal.Kind == proactivity.GoalVisitPark && res.Decision.Interrupt {
		t.Fatal("work is not the park scenario")
	}
}

func TestPolicyRejectsBadHours(t *testing.T) {
	err := proactivity.Policy{MinScore: 70, QuietStart: 25, QuietEnd: 8}.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestFormatResultMentionsFixture(t *testing.T) {
	s := sensors(
		weather.FixtureClear,
		situation.MustMock(situation.PlaceHome, situation.ActivityIdle),
		calendar.NewMock(),
		saturdayAfternoon(),
	)
	res, err := proactivity.Tick(context.Background(), s, proactivity.DefaultPolicy(), proactivity.NewMemory())
	if err != nil {
		t.Fatal(err)
	}
	text := proactivity.FormatResult(res, 1)
	for _, want := range []string{"FIXTURE weather", "MOCK location", "visit_park", "Interrupt: YES"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q\n%s", want, text)
		}
	}
}

func TestMissingSensorRejected(t *testing.T) {
	_, err := proactivity.Tick(context.Background(), proactivity.Sensors{
		Clock: saturdayAfternoon(),
	}, proactivity.DefaultPolicy(), nil)
	if err == nil {
		t.Fatal("expected error")
	}
}
