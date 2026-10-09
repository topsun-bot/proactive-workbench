package proactivity

import (
	"time"

	"github.com/topsun-bot/proactive-workbench/internal/datasources/calendar"
	"github.com/topsun-bot/proactive-workbench/internal/datasources/situation"
	"github.com/topsun-bot/proactive-workbench/internal/datasources/weather"
)

func GenerateGoal(p Perception) Goal {
	park := scorePark(p)
	umbrella := scoreUmbrella(p)
	switch {
	case park.Score >= umbrella.Score && park.Score > 0:
		return park
	case umbrella.Score > 0:
		return umbrella
	default:
		return Goal{
			Kind:   GoalNone,
			Title:  "No proactive goal",
			Reason: "Nothing in the current fixtures crosses the usefulness bar.",
			Score:  0,
		}
	}
}

func scorePark(p Perception) Goal {
	g := Goal{
		Kind:        GoalVisitPark,
		Title:       "Go to the park",
		WindowStart: p.At,
	}
	if p.Situation.Place != situation.PlaceHome {
		g.Reason = "Not at home."
		return g
	}
	g.Score += 30
	if p.Situation.Activity != situation.ActivityIdle {
		g.Reason = "User is busy."
		return g
	}
	g.Score += 20
	if !weather.OutdoorComfort(p.NowWeather) {
		g.Reason = "Weather is not outdoor-OK (" + p.NowWeather.Condition.DisplayName() + ")."
		return g
	}
	g.Score += 30
	hour := p.At.Hour()
	weekend := p.At.Weekday() == time.Saturday || p.At.Weekday() == time.Sunday
	if weekend || hour >= 16 {
		g.Score += 15
	}
	if ev, busy := calendar.NextBusy(p.Events, p.At, 2*time.Hour); busy {
		g.Score -= 50
		g.Reason = "Meeting soon: " + ev.Title
		return g
	}
	g.Score += 15
	g.Reason = "At home, idle, weather is good, calendar is free."
	return g
}

func scoreUmbrella(p Perception) Goal {
	g := Goal{
		Kind:        GoalUmbrellaReminder,
		Title:       "Remind to bring an umbrella tomorrow 08:00",
		WindowStart: p.TomorrowAM.At,
	}
	if !p.TomorrowAM.Condition.NeedsUmbrella() {
		g.Reason = "Tomorrow 08:00 is not rain/storm in the fixture."
		return g
	}
	g.Score += 50
	if calendar.HasUmbrellaEvent(p.Events) {
		g.Reason = "An umbrella reminder already exists."
		return g
	}
	g.Score += 30
	if p.At.Hour() < 21 {
		g.Score += 10
	}
	g.Reason = "Rain or storm tomorrow morning; no umbrella event on the calendar."
	return g
}
