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
	GoalCommitmentNudge  GoalKind = "commitment_nudge"
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
	// CalendarError is set when ICS is missing or unreadable. Events stay empty.
	CalendarError       string
	CalendarUserMessage string
}

type Decision struct {
	// Propose is the core’s first gate (quiet hours, score, dedupe).
	// It is not a final “show a banner” / notify decision — the client
	// applies OS Focus / Do Not Disturb as a second gate.
	Propose bool
	// Interrupt is a legacy alias of Propose for existing CLI tests.
	// It is not a notify decision.
	Interrupt   bool
	Reason      string
	Fingerprint string
	Score       int
}

// FirstGate is the single source of truth for Snapshot.propose / Snapshot.reason.
// Shaoruru’s /api/today encoder must copy these fields and must not recompute
// quiet hours, score, or dedupe. Call FirstGateFrom after Core.Tick.
type FirstGate struct {
	Propose bool   `json:"propose"`
	Reason  string `json:"reason"`
}

// FirstGate returns the first-gate fields for a Snapshot suggestion.
func (d Decision) FirstGate() FirstGate {
	return FirstGate{Propose: d.Propose, Reason: d.Reason}
}

// FirstGateFrom is the exported hook for the workbench /api/today Snapshot
// encoder: gate := proactivity.FirstGateFrom(result).
func FirstGateFrom(r Result) FirstGate {
	return r.Decision.FirstGate()
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
