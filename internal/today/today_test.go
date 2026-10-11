package today

import (
	"strings"
	"testing"
	"time"

	"github.com/topsun-bot/proactive-workbench/internal/tools/weather"
)

func TestBuildWithoutWeatherFixtureDoesNotInventClear(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 11, 9, 0, 0, 0, loc)
	snap := Build(Input{Now: now}, LongTerm{})
	if snap.WeatherMock {
		t.Fatal("no weather fixture should not be labeled MOCK weather")
	}
	if snap.WeatherAvailable {
		t.Fatal("WeatherAvailable must be false without a live reading")
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
	now := time.Date(2026, 10, 10, 7, 15, 0, 0, loc)
	core, err := NewServeCore(now, loc, weather.Rain)
	if err != nil {
		t.Fatal(err)
	}
	text := Format(Build(Input{Now: now, Weather: weather.Rain, Core: core, DebugFixture: true}, Fixture()))
	for _, want := range []string{"Morning walk", "Team standup", "Alex", "MOCK", "propose="} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q\n%s", want, text)
		}
	}
}
