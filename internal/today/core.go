package today

import (
	"context"
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
	var wxSrc dsweather.Source
	if wx == "" || wx == toolweather.Unavailable {
		wxSrc = unavailableWeather{loc: dsweather.DefaultLocation}
	} else {
		fixture := weatherFixtureName(wx)
		mock, err := dsweather.NewMock(fixture)
		if err != nil {
			return nil, fmt.Errorf("today: weather fixture: %w", err)
		}
		mock.SetLocationTZ(loc)
		wxSrc = mock
	}
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
	return NewServeCoreFromSensors(proactivity.Sensors{
		Weather:   wxSrc,
		Calendar:  calendar.NewMock(),
		Situation: sit,
		Clock:     clk,
		Location:  dsweather.DefaultLocation,
	}, store)
}

// NewServeCoreFromSensors is the live (or test-injected) Core constructor.
// store may be nil; a fixture-seeded memory store is used then (same as serve).
func NewServeCoreFromSensors(s proactivity.Sensors, store memory.Store) (*proactivity.Core, error) {
	if store == nil {
		ms := memory.NewMemStore()
		if _, err := ms.SeedFixture(); err != nil {
			return nil, fmt.Errorf("today: memory fixture: %w", err)
		}
		store = ms
	}
	return proactivity.NewCore(s, proactivity.DefaultPolicy(), store)
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

type unavailableWeather struct {
	loc dsweather.Location
}

func (u unavailableWeather) Snapshot(_ context.Context, _ dsweather.Location, _ time.Time) (dsweather.Snapshot, error) {
	return dsweather.UnavailableSnapshot(u.loc, "no live weather client in this mux"), nil
}
