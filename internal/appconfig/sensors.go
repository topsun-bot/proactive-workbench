package appconfig

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/topsun-bot/proactive-workbench/internal/clock"
	"github.com/topsun-bot/proactive-workbench/internal/datasources/calendar"
	"github.com/topsun-bot/proactive-workbench/internal/datasources/situation"
	"github.com/topsun-bot/proactive-workbench/internal/datasources/weather"
	"github.com/topsun-bot/proactive-workbench/internal/proactivity"
)

// SensorOpts builds proactivity.Sensors for the CLI / workbench serve.
// Unit tests inject WeatherDoer (never nil when exercising the live path).
// Production live mode may pass a nil Doer; Sensors then uses NewLiveOpenMeteo.
type SensorOpts struct {
	Config          Config
	DebugFixture    bool
	WeatherFixture  string
	CalendarFixture bool
	Place           situation.Place
	Activity        situation.Activity
	Now             time.Time
	TZ              *time.Location
	WeatherDoer     weather.Doer
	WeatherBaseURL  string
}

// UseFixtures reports whether labeled fixtures should be used.
func (o SensorOpts) UseFixtures() bool {
	return o.DebugFixture || o.Config.DebugFixture ||
		strings.TrimSpace(o.WeatherFixture) != "" ||
		o.CalendarFixture
}

// Sensors constructs weather + calendar + situation + clock.
// Live weather never falls back to fixture numbers. Live calendar never
// invents events: a missing ics_path is calendar.Unconfigured.
func Sensors(o SensorOpts) (proactivity.Sensors, error) {
	tz := o.TZ
	if tz == nil {
		name := o.Config.Timezone
		if name == "" {
			name = DefaultTimezone
		}
		loaded, err := time.LoadLocation(name)
		if err != nil {
			return proactivity.Sensors{}, fmt.Errorf("appconfig: timezone %q: %w", name, err)
		}
		tz = loaded
	}

	place := o.Place
	if place == "" {
		place = situation.PlaceHome
	}
	act := o.Activity
	if act == "" {
		act = situation.ActivityIdle
	}
	sit, err := situation.NewMock(place, act)
	if err != nil {
		return proactivity.Sensors{}, err
	}

	var clk clock.Clock
	if !o.Now.IsZero() {
		clk = clock.Fixed(o.Now, tz)
	} else {
		clk = clock.Live(tz)
	}

	loc := o.Config.Location()

	wx, err := weatherSource(o, loc, tz)
	if err != nil {
		return proactivity.Sensors{}, err
	}
	cal, err := calendarSource(o, tz)
	if err != nil {
		return proactivity.Sensors{}, err
	}

	return proactivity.Sensors{
		Weather:   wx,
		Calendar:  cal,
		Situation: sit,
		Clock:     clk,
		Location:  loc,
	}, nil
}

func weatherSource(o SensorOpts, loc weather.Location, tz *time.Location) (weather.Source, error) {
	if o.UseFixtures() {
		name := o.WeatherFixture
		if name == "" {
			name = weather.FixtureClear
		}
		if name == string(weather.ConditionUnavailable) {
			return unavailableWeather{loc: loc}, nil
		}
		mock, err := weather.NewMock(name)
		if err != nil {
			return nil, err
		}
		mock.SetLocation(loc)
		mock.SetLocationTZ(tz)
		return mock, nil
	}
	if o.WeatherDoer == nil {
		live, err := weather.NewLiveOpenMeteo()
		if err != nil {
			return nil, err
		}
		live.Timezone = tz.String()
		return live, nil
	}
	base := o.WeatherBaseURL
	if base == "" {
		base = "http://weather.test.invalid"
	}
	om, err := weather.NewOpenMeteo(o.WeatherDoer, base)
	if err != nil {
		return nil, err
	}
	om.Timezone = tz.String()
	return om, nil
}

func calendarSource(o SensorOpts, tz *time.Location) (calendar.Source, error) {
	if o.UseFixtures() {
		if o.CalendarFixture {
			return calendar.MustFixtureMock(), nil
		}
		// Fixture mode without --calendar-fixture: empty mock. No invented events.
		return calendar.NewMock(), nil
	}
	path := strings.TrimSpace(o.Config.ICSPath)
	if path == "" {
		return calendar.NewUnconfigured(), nil
	}
	ics, err := calendar.NewICS(path)
	if err != nil {
		return nil, err
	}
	ics.Location = tz
	return ics, nil
}

// unavailableWeather is the --debug-fixture --weather=unavailable source.
type unavailableWeather struct {
	loc weather.Location
}

func (u unavailableWeather) Snapshot(_ context.Context, _ weather.Location, _ time.Time) (weather.Snapshot, error) {
	return weather.UnavailableSnapshot(u.loc, "debug-fixture weather=unavailable"), nil
}
