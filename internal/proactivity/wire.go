package proactivity

import (
	"time"

	"github.com/topsun-bot/proactive-workbench/internal/datasources/weather"
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
	At                  string      `json:"at"`
	Place               string      `json:"place"`
	Activity            string      `json:"activity"`
	WeatherCondition    string      `json:"weatherCondition"`
	WeatherTempC        float64     `json:"weatherTempC"`
	WeatherSource       string      `json:"weatherSource"`
	WeatherAvailable    bool        `json:"weatherAvailable"`
	WeatherError        string      `json:"weatherError,omitempty"`
	WeatherUserMessage  string      `json:"weatherUserMessage,omitempty"`
	CalendarCount       int         `json:"calendarCount"`
	CalendarAvailable   bool        `json:"calendarAvailable"`
	CalendarError       string      `json:"calendarError,omitempty"`
	CalendarUserMessage string      `json:"calendarUserMessage,omitempty"`
	Events              []WireEvent `json:"events"`
	GoalKind            string      `json:"goalKind"`
	GoalTitle           string      `json:"goalTitle"`
	GoalReason          string      `json:"goalReason"`
	GoalScore           int         `json:"goalScore"`
	PlanSteps           []string    `json:"planSteps"`
	Propose             bool        `json:"propose"`
	Reason              string      `json:"reason"`
	Interrupt           bool        `json:"interrupt"`
	DecisionReason      string      `json:"decisionReason"`
	Fingerprint         string      `json:"fingerprint"`
	SourcesAreFixtures  bool        `json:"sourcesAreFixtures"`
}

// WireSuggestion is one item in GET /v1/today data.suggestions[].
// Propose and Reason are copied from FirstGateFrom (do not recompute).
type WireSuggestion struct {
	Title   string `json:"title"`
	Body    string `json:"body"`
	Kind    string `json:"kind"`
	Source  string `json:"source"`
	Propose bool   `json:"propose"`
	Reason  string `json:"reason"`
}

// WireToday is the core Today contract. GET /v1/today returns it inside
// the envelope. /api/today is owned by workbench serve (UI Snapshot).
type WireToday struct {
	At                  string           `json:"at"`
	Place               string           `json:"place"`
	Activity            string           `json:"activity"`
	LocationSource      string           `json:"locationSource"`
	WeatherCondition    string           `json:"weatherCondition"`
	WeatherSource       string           `json:"weatherSource"`
	WeatherAvailable    bool             `json:"weatherAvailable"`
	WeatherError        string           `json:"weatherError,omitempty"`
	WeatherUserMessage  string           `json:"weatherUserMessage,omitempty"`
	CalendarAvailable   bool             `json:"calendarAvailable"`
	CalendarError       string           `json:"calendarError,omitempty"`
	CalendarUserMessage string           `json:"calendarUserMessage,omitempty"`
	GoalKind            string           `json:"goalKind"`
	Brief               *Brief           `json:"brief,omitempty"`
	Suggestions         []WireSuggestion `json:"suggestions"`
	SourcesAreFixtures  bool             `json:"sourcesAreFixtures"`
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
		At:                  r.Perception.At.Format(time.RFC3339),
		Place:               string(r.Perception.Situation.Place),
		Activity:            string(r.Perception.Situation.Activity),
		WeatherCondition:    string(r.Perception.NowWeather.Condition),
		WeatherTempC:        r.Perception.NowWeather.TemperatureC,
		WeatherSource:       r.Perception.Weather.Source,
		WeatherAvailable:    r.Perception.Weather.Available(),
		WeatherError:        r.Perception.Weather.Error,
		WeatherUserMessage:  weatherUserMessage(r.Perception),
		CalendarCount:       len(r.Perception.Events),
		CalendarAvailable:   r.Perception.CalendarError == "",
		CalendarError:       r.Perception.CalendarError,
		CalendarUserMessage: r.Perception.CalendarUserMessage,
		Events:              events,
		GoalKind:            string(r.Goal.Kind),
		GoalTitle:           r.Goal.Title,
		GoalReason:          r.Goal.Reason,
		GoalScore:           r.Goal.Score,
		PlanSteps:           r.Plan.Steps,
		Propose:             r.Decision.Propose,
		Reason:              r.Decision.Reason,
		Interrupt:           r.Decision.Interrupt,
		DecisionReason:      r.Decision.Reason,
		Fingerprint:         r.Decision.Fingerprint,
		SourcesAreFixtures:  r.Perception.Weather.IsMock || r.Perception.Situation.IsMock,
	}
}

const suggestionGateSource = "proactivity.Core first gate (quiet hours / score / dedupe)"

func ResultToSuggestion(r Result) WireSuggestion {
	gate := FirstGateFrom(r)
	return WireSuggestion{
		Title:   r.Goal.Title,
		Body:    r.Goal.Reason,
		Kind:    string(r.Goal.Kind),
		Source:  suggestionGateSource,
		Propose: gate.Propose,
		Reason:  gate.Reason,
	}
}

func ResultToToday(r Result, brief Brief) WireToday {
	return WireToday{
		At:                  r.Perception.At.Format(time.RFC3339),
		Place:               string(r.Perception.Situation.Place),
		Activity:            string(r.Perception.Situation.Activity),
		LocationSource:      r.Perception.Situation.Source,
		WeatherCondition:    string(r.Perception.NowWeather.Condition),
		WeatherSource:       r.Perception.Weather.Source,
		WeatherAvailable:    r.Perception.Weather.Available(),
		WeatherError:        r.Perception.Weather.Error,
		WeatherUserMessage:  weatherUserMessage(r.Perception),
		CalendarAvailable:   r.Perception.CalendarError == "",
		CalendarError:       r.Perception.CalendarError,
		CalendarUserMessage: r.Perception.CalendarUserMessage,
		GoalKind:            string(r.Goal.Kind),
		Brief:               &brief,
		Suggestions:         []WireSuggestion{ResultToSuggestion(r)},
		SourcesAreFixtures:  r.Perception.Weather.IsMock || r.Perception.Situation.IsMock,
	}
}

func weatherUserMessage(p Perception) string {
	if p.Weather.Available() {
		return ""
	}
	return weather.UserFacingUnavailable
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
