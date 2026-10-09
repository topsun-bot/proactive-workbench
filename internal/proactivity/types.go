package proactivity

import (
	"time"

	"github.com/topsun-bot/proactive-workbench/internal/datasources/calendar"
	"github.com/topsun-bot/proactive-workbench/internal/datasources/situation"
	"github.com/topsun-bot/proactive-workbench/internal/datasources/weather"
)

type GoalKind string

const (
	GoalNone             GoalKind = "none"
	GoalVisitPark        GoalKind = "visit_park"
	GoalUmbrellaReminder GoalKind = "umbrella_reminder"
)

type Goal struct {
	Kind        GoalKind
	Title       string
	Reason      string
	Score       int
	WindowStart time.Time
}

type Plan struct {
	Goal  Goal
	Steps []string
}

type Perception struct {
	At         time.Time
	Situation  situation.Snapshot
	Weather    weather.Snapshot
	NowWeather weather.HourlyPoint
	TomorrowAM weather.HourlyPoint
	Events     []calendar.Event
	Reminders  []calendar.Reminder
}

type Decision struct {
	Interrupt   bool
	Reason      string
	Fingerprint string
	Score       int
}

type Result struct {
	Perception Perception
	Goal       Goal
	Plan       Plan
	Decision   Decision
}

func (g Goal) IsNone() bool {
	return g.Kind == GoalNone || g.Kind == ""
}
