package proactivity

import (
	"time"

	"github.com/topsun-bot/proactive-workbench/internal/memory"
)

// Envelope is the language-neutral JSON wrapper for every HTTP and --json reply.
type Envelope struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	Data  any    `json:"data,omitempty"`
}

type WireEvent struct {
	UID   string `json:"uid"`
	Title string `json:"title"`
	Start string `json:"start"`
}

type WireTick struct {
	At                 string      `json:"at"`
	Place              string      `json:"place"`
	Activity           string      `json:"activity"`
	WeatherCondition   string      `json:"weatherCondition"`
	WeatherTempC       float64     `json:"weatherTempC"`
	WeatherSource      string      `json:"weatherSource"`
	CalendarCount      int         `json:"calendarCount"`
	Events             []WireEvent `json:"events"`
	GoalKind           string      `json:"goalKind"`
	GoalTitle          string      `json:"goalTitle"`
	GoalReason         string      `json:"goalReason"`
	GoalScore          int         `json:"goalScore"`
	PlanSteps          []string    `json:"planSteps"`
	Interrupt          bool        `json:"interrupt"`
	DecisionReason     string      `json:"decisionReason"`
	Fingerprint        string      `json:"fingerprint"`
	SourcesAreFixtures bool        `json:"sourcesAreFixtures"`
}

type WireHealth struct {
	Service string `json:"service"`
	API     int    `json:"api"`
	Bound   string `json:"bound"`
}

type WireRoutineRun struct {
	Kind  RoutineKind `json:"kind"`
	At    string      `json:"at"`
	Brief *Brief      `json:"brief,omitempty"`
	Tick  *WireTick   `json:"tick,omitempty"`
}

func ResultToWire(r Result) WireTick {
	events := make([]WireEvent, 0, len(r.Perception.Events))
	for _, ev := range r.Perception.Events {
		events = append(events, WireEvent{
			UID:   ev.UID,
			Title: ev.Title,
			Start: ev.Start.Format(time.RFC3339),
		})
	}
	return WireTick{
		At:                 r.Perception.At.Format(time.RFC3339),
		Place:              string(r.Perception.Situation.Place),
		Activity:           string(r.Perception.Situation.Activity),
		WeatherCondition:   string(r.Perception.NowWeather.Condition),
		WeatherTempC:       r.Perception.NowWeather.TemperatureC,
		WeatherSource:      r.Perception.Weather.Source,
		CalendarCount:      len(r.Perception.Events),
		Events:             events,
		GoalKind:           string(r.Goal.Kind),
		GoalTitle:          r.Goal.Title,
		GoalReason:         r.Goal.Reason,
		GoalScore:          r.Goal.Score,
		PlanSteps:          r.Plan.Steps,
		Interrupt:          r.Decision.Interrupt,
		DecisionReason:     r.Decision.Reason,
		Fingerprint:        r.Decision.Fingerprint,
		SourcesAreFixtures: r.Perception.Weather.IsMock || r.Perception.Situation.IsMock,
	}
}

func RoutineRunsToWire(runs []RoutineRun) []WireRoutineRun {
	out := make([]WireRoutineRun, 0, len(runs))
	for _, r := range runs {
		w := WireRoutineRun{Kind: r.Kind, At: r.At, Brief: r.Brief}
		if r.Tick != nil {
			t := ResultToWire(*r.Tick)
			w.Tick = &t
		}
		out = append(out, w)
	}
	return out
}

// WireMemory is the on-disk snapshot as returned over JSON (same schema as the file).
type WireMemory = memory.Snapshot
