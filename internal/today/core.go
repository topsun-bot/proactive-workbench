package today

import (
	"fmt"
	"time"

	"github.com/topsun-bot/proactive-workbench/internal/clock"
	"github.com/topsun-bot/proactive-workbench/internal/datasources/calendar"
	"github.com/topsun-bot/proactive-workbench/internal/datasources/situation"
	dsweather "github.com/topsun-bot/proactive-workbench/internal/datasources/weather"
	"github.com/topsun-bot/proactive-workbench/internal/memory"
	"github.com/topsun-bot/proactive-workbench/internal/proactivity"
	toolweather "github.com/topsun-bot/proactive-workbench/internal/tools/weather"
)

// NewServeCore builds the PR #3 proactivity.Core used by workbench serve.
// Gate 1 (propose/reason) lives only inside that Core.
func NewServeCore(now time.Time, loc *time.Location, wx toolweather.Condition) (*proactivity.Core, error) {
	if loc == nil {
		loc = time.UTC
	}
	fixture := weatherFixtureName(wx)
	wxSrc, err := dsweather.NewMock(fixture)
	if err != nil {
		return nil, fmt.Errorf("today: weather fixture: %w", err)
	}
	wxSrc.SetLocationTZ(loc)
	sit, err := situation.NewMock(situation.PlaceHome, situation.ActivityIdle)
	if err != nil {
		return nil, fmt.Errorf("today: situation fixture: %w", err)
	}
	var clk clock.Clock
	if now.IsZero() {
		clk = clock.Live(loc)
	} else {
		clk = clock.Fixed(now, loc)
	}
	store := memory.NewMemStore()
	if _, err := store.SeedFixture(); err != nil {
		return nil, fmt.Errorf("today: memory fixture: %w", err)
	}
	return proactivity.NewCore(proactivity.Sensors{
		Weather:   wxSrc,
		Calendar:  calendar.NewMock(),
		Situation: sit,
		Clock:     clk,
		Location:  dsweather.DefaultLocation,
	}, proactivity.DefaultPolicy(), store)
}

func weatherFixtureName(wx toolweather.Condition) string {
	switch wx {
	case toolweather.Rain:
		return dsweather.FixtureRain
	case toolweather.Cloudy:
		return dsweather.FixtureCloudy
	case toolweather.Clear:
		return dsweather.FixtureClear
	default:
		// Live default has no MOCK weather scenario. Do not invent clear
		// outdoor-OK weather for the core — cloudy is not OutdoorOK.
		return dsweather.FixtureCloudy
	}
}
