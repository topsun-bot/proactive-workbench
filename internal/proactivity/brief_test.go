package proactivity_test

import (
	"context"
	"strings"
	"testing"

	"github.com/topsun-bot/proactive-workbench/internal/datasources/calendar"
	"github.com/topsun-bot/proactive-workbench/internal/datasources/situation"
	"github.com/topsun-bot/proactive-workbench/internal/datasources/weather"
	"github.com/topsun-bot/proactive-workbench/internal/memory"
	"github.com/topsun-bot/proactive-workbench/internal/proactivity"
)

func TestBriefLabelsFixtures(t *testing.T) {
	s := sensors(
		weather.FixtureClear,
		situation.MustMock(situation.PlaceHome, situation.ActivityIdle),
		calendar.NewMock(),
		saturdayAfternoon(),
	)
	p, err := proactivity.Perceive(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	b := proactivity.BuildBrief(p, memory.FixtureSnapshot(), proactivity.GenerateGoal(p, memory.FixtureSnapshot()))
	if !b.IsFixture || !strings.Contains(b.Source, "FIXTURE") {
		t.Fatalf("brief must be labeled fixture: %#v", b)
	}
	text := proactivity.FormatBrief(b)
	for _, want := range []string{"Morning brief", "FIXTURE", "Clear", "Spend more time outdoors", "visit_park"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q\n%s", want, text)
		}
	}
}
