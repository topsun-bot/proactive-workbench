package proactivity

import (
	"strings"
	"time"

	"github.com/topsun-bot/proactive-workbench/internal/datasources/calendar"
	"github.com/topsun-bot/proactive-workbench/internal/datasources/situation"
	"github.com/topsun-bot/proactive-workbench/internal/datasources/weather"
	"github.com/topsun-bot/proactive-workbench/internal/memory"
)

func GenerateGoal(p Perception, snap memory.Snapshot) Goal {
	park := scorePark(p, snap)
	umbrella := scoreUmbrella(p)
	commit := scoreCommitment(p, snap)
	best := Goal{
		Kind:   GoalNone,
		Title:  "No proactive goal",
		Reason: "Nothing in the current fixtures crosses the usefulness bar.",
		Score:  0,
	}
	for _, g := range []Goal{park, umbrella, commit} {
		if g.Score > best.Score {
			best = g
		}
	}
	return best
}

func scorePark(p Perception, snap memory.Snapshot) Goal {
	g := Goal{
		Kind:        GoalVisitPark,
		Title:       "Go to the park",
		WindowStart: p.At,
	}
	if !snap.LikesOutdoors() {
		g.Reason = "Preference likes_outdoors is false."
		return g
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
	if outdoorGoal(snap) {
		g.Score += 5
		g.Reason += " Active long-term goal mentions outdoors."
	}
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

func scoreCommitment(p Perception, snap memory.Snapshot) Goal {
	c, ok := snap.NextCommitment(p.At, 2*time.Hour)
	if !ok {
		return Goal{Kind: GoalCommitmentNudge}
	}
	g := Goal{
		Kind:        GoalCommitmentNudge,
		Title:       "Upcoming: " + c.Title,
		WindowStart: c.When,
		Score:       80,
		Reason:      "Memory commitment within 2 hours: " + c.Title,
	}
	if person, found := snap.PersonNamedIn(c.Title + " " + c.Notes); found {
		g.Reason += " (involves " + person.Name + ")"
		g.Score += 5
	}
	return g
}

func outdoorGoal(snap memory.Snapshot) bool {
	for _, g := range snap.ActiveGoals() {
		t := strings.ToLower(g.Title + " " + g.Notes)
		if strings.Contains(t, "outdoor") || strings.Contains(t, "park") || strings.Contains(t, "walk") {
			return true
		}
	}
	return false
}
