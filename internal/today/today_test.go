package today

import (
	"strings"
	"testing"
	"time"

	"github.com/topsun-bot/proactive-workbench/internal/memory"
	"github.com/topsun-bot/proactive-workbench/internal/tools/weather"
)

func TestBuildSuggestsMovingWalk(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 10, 7, 15, 0, 0, loc)
	snap := Build(Input{Now: now, Weather: weather.Clear}, memory.Fixture())
	if !strings.Contains(snap.Briefing, "Good morning") {
		t.Fatalf("briefing: %s", snap.Briefing)
	}
	if !snap.WeatherMock || !strings.Contains(snap.MemorySource, "MOCK") {
		t.Fatal("memory and weather must be labeled MOCK")
	}
	if len(snap.Suggestions) == 0 {
		t.Fatal("expected unprompted suggestion")
	}
	found := false
	for _, s := range snap.Suggestions {
		if strings.Contains(s.Body, "slept little last night") && strings.Contains(s.Body, "10am meeting") {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing walk-to-evening suggestion: %+v", snap.Suggestions)
	}
}

func TestBuildWithoutWeatherFixtureDoesNotInventClear(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 11, 9, 0, 0, 0, loc)
	snap := Build(Input{Now: now}, memory.Fixture())
	if snap.WeatherMock {
		t.Fatal("no weather fixture should not be labeled MOCK weather")
	}
	if strings.Contains(snap.WeatherLine, "Clear") || strings.Contains(snap.WeatherLine, "22°C") {
		t.Fatalf("invented clear forecast: %q", snap.WeatherLine)
	}
	if !strings.Contains(snap.Briefing, "Weather unavailable") {
		t.Fatalf("briefing: %s", snap.Briefing)
	}
}

func TestFormatListsRoutinesAndPeople(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	text := Format(Build(Input{Now: time.Date(2026, 10, 10, 7, 15, 0, 0, loc), Weather: weather.Rain}, memory.Fixture()))
	for _, want := range []string{"Morning walk", "Team standup", "Alex", "MOCK", "Pack an umbrella"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q\n%s", want, text)
		}
	}
}
