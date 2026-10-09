package proactivity

import (
	"context"
	"fmt"
	"time"

	"github.com/topsun-bot/proactive-workbench/internal/clock"
	"github.com/topsun-bot/proactive-workbench/internal/datasources/calendar"
	"github.com/topsun-bot/proactive-workbench/internal/datasources/situation"
	"github.com/topsun-bot/proactive-workbench/internal/datasources/weather"
)

type Sensors struct {
	Weather   weather.Source
	Calendar  calendar.Source
	Situation situation.Source
	Clock     clock.Clock
	Location  weather.Location
}

func (s Sensors) valid() error {
	if s.Weather == nil {
		return fmt.Errorf("proactivity: weather source is required")
	}
	if s.Calendar == nil {
		return fmt.Errorf("proactivity: calendar source is required")
	}
	if s.Situation == nil {
		return fmt.Errorf("proactivity: situation source is required")
	}
	if s.Clock.Now == nil {
		return fmt.Errorf("proactivity: clock is required")
	}
	if s.Clock.Location == nil {
		return fmt.Errorf("proactivity: clock location is required")
	}
	return nil
}

func Perceive(ctx context.Context, s Sensors) (Perception, error) {
	if err := s.valid(); err != nil {
		return Perception{}, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	at := s.Clock.Now().In(s.Clock.Location)
	loc := s.Location
	if loc.Label == "" && loc.Latitude == 0 && loc.Longitude == 0 {
		loc = weather.DefaultLocation
	}

	sit, err := s.Situation.Snapshot(ctx, at)
	if err != nil {
		return Perception{}, fmt.Errorf("proactivity: situation: %w", err)
	}
	wx, err := s.Weather.Snapshot(ctx, loc, at)
	if err != nil {
		return Perception{}, fmt.Errorf("proactivity: weather: %w", err)
	}
	nowWx := wx.Current
	if p, ok := wx.PointAt(at); ok {
		nowWx = p
	}

	tomorrowAM, err := s.Clock.NextMorning(8, 0)
	if err != nil {
		return Perception{}, err
	}
	amWx := nowWx
	if p, ok := wx.PointAt(tomorrowAM); ok {
		amWx = p
	}

	win := calendar.Window{From: at, To: at.Add(24 * time.Hour)}
	events, err := s.Calendar.ListEvents(ctx, win)
	if err != nil {
		return Perception{}, fmt.Errorf("proactivity: calendar events: %w", err)
	}
	reminders, err := s.Calendar.ListReminders(ctx, win)
	if err != nil {
		return Perception{}, fmt.Errorf("proactivity: calendar reminders: %w", err)
	}

	return Perception{
		At:         at,
		Situation:  sit,
		Weather:    wx,
		NowWeather: nowWx,
		TomorrowAM: amWx,
		Events:     events,
		Reminders:  reminders,
	}, nil
}
