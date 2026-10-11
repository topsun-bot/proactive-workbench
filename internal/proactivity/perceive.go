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
		// Live fetch failed: surface unavailable. Never substitute fixture numbers.
		wx = weather.UnavailableSnapshot(loc, err.Error())
	}
	nowWx := wx.Current
	if wx.Available() {
		if p, ok := wx.PointAt(at); ok {
			nowWx = p
		}
	}

	tomorrowAM, err := s.Clock.NextMorning(8, 0)
	if err != nil {
		return Perception{}, err
	}
	amWx := nowWx
	if wx.Available() {
		if p, ok := wx.PointAt(tomorrowAM); ok {
			amWx = p
		}
	}

	win := calendar.Window{From: at, To: at.Add(24 * time.Hour)}
	events, evErr := s.Calendar.ListEvents(ctx, win)
	reminders, remErr := s.Calendar.ListReminders(ctx, win)
	calErr := evErr
	if calErr == nil {
		calErr = remErr
	}
	if calErr != nil {
		events = nil
		reminders = nil
	}

	p := Perception{
		At:         at,
		Situation:  sit,
		Weather:    wx,
		NowWeather: nowWx,
		TomorrowAM: amWx,
		Events:     events,
		Reminders:  reminders,
	}
	if calErr != nil {
		p.CalendarError = calErr.Error()
		p.CalendarUserMessage = calendar.UserMessageForError(calErr)
	}
	return p, nil
}
