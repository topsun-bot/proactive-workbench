package today

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/topsun-bot/proactive-workbench/internal/proactivity"
	"github.com/topsun-bot/proactive-workbench/internal/tools/weather"
)

type Suggestion struct {
	Title   string
	Body    string
	Kind    string
	Source  string
	Propose bool   `json:"propose"`
	Reason  string `json:"reason"`
}

type Signal struct {
	Label string
	Value string
	Mock  bool
}

type Snapshot struct {
	Greeting     string
	DateLabel    string
	Timezone     string
	Briefing     string
	WeatherLine  string
	WeatherMock  bool
	SleepLine    string
	People       []Person
	Preferences  []Preference
	Goals        []Goal
	Tasks        []Task
	Routines     []Routine
	Signals      []Signal
	Suggestions  []Suggestion
	MemorySource string
}

type Input struct {
	Now     time.Time
	Weather weather.Condition
	// Core is the PR #3 gate-1 source of truth. Suggestions copy propose/reason
	// from Core.Today — they are not recomputed here.
	Core *proactivity.Core
}

func Build(in Input, mem LongTerm) Snapshot {
	if mem.SourceLabel == "" {
		mem = Fixture()
	}
	loc := in.Now.Location()
	if loc == nil {
		loc = time.UTC
	}
	wx := in.Weather
	weatherMock := wx != "" && wx != weather.Unavailable
	weatherLine := "unavailable (no MOCK scenario)"
	if weatherMock {
		weatherLine = fmt.Sprintf("%s, %d°C, precip %d%%", wx.DisplayName(), int(wx.TemperatureC()), wx.PrecipPct())
	}
	s := Snapshot{
		Greeting:     greeting(in.Now),
		DateLabel:    in.Now.Format("Monday, 2 January 2006"),
		Timezone:     loc.String(),
		WeatherLine:  weatherLine,
		WeatherMock:  weatherMock,
		SleepLine:    mem.SleepNote,
		People:       mem.People,
		Preferences:  mem.Preferences,
		Goals:        mem.Goals,
		Tasks:        mem.Tasks,
		Routines:     mem.Routines,
		MemorySource: mem.SourceLabel,
	}
	weatherBrief := s.WeatherLine
	if weatherMock {
		weatherBrief = "MOCK weather is " + s.WeatherLine
	} else {
		weatherBrief = "Weather " + s.WeatherLine
	}
	s.Briefing = fmt.Sprintf(
		"%s %s. %s. %s. Next meeting is standup with Sam at 10:00.",
		s.Greeting, s.DateLabel, weatherBrief, mem.SleepNote,
	)
	s.Signals = []Signal{
		{Label: "Sleep", Value: fmt.Sprintf("%.1fh last night", mem.SleepHours), Mock: true},
		{Label: "Weather", Value: s.WeatherLine, Mock: weatherMock},
		{Label: "Next meeting", Value: "10:00 standup with Sam", Mock: true},
		{Label: "People nearby", Value: peopleLine(mem.People), Mock: true},
	}
	s.Suggestions = suggestionsFromCore(in.Core)
	return s
}

func greeting(now time.Time) string {
	switch {
	case now.Hour() < 12:
		return "Good morning."
	case now.Hour() < 18:
		return "Good afternoon."
	default:
		return "Good evening."
	}
}

func peopleLine(people []Person) string {
	names := make([]string, 0, len(people))
	for _, p := range people {
		names = append(names, p.Name+" ("+p.Relation+")")
	}
	return strings.Join(names, ", ")
}

// suggestionsFromCore copies FirstGateFrom after Core.Tick. Gate 1
// (quiet hours / score / dedupe) is not recomputed in this package.
func suggestionsFromCore(core *proactivity.Core) []Suggestion {
	if core == nil {
		return nil
	}
	res, err := core.Tick(context.Background())
	if err != nil {
		return nil
	}
	gate := proactivity.FirstGateFrom(res)
	w := proactivity.ResultToSuggestion(res)
	return []Suggestion{{
		Title:   w.Title,
		Body:    w.Body,
		Kind:    w.Kind,
		Source:  w.Source,
		Propose: gate.Propose,
		Reason:  gate.Reason,
	}}
}

func Format(s Snapshot) string {
	var b strings.Builder
	fmt.Fprintln(&b, "Today")
	fmt.Fprintln(&b, "====")
	fmt.Fprintf(&b, "%s\n", s.Briefing)
	fmt.Fprintf(&b, "Memory:  %s\n", s.MemorySource)
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Tasks")
	for _, t := range s.Tasks {
		fmt.Fprintf(&b, "  - %02d:%02d  %s (%s)\n", t.Hour, t.Minute, t.Title, t.Kind)
	}
	fmt.Fprintln(&b, "Signals")
	for _, sig := range s.Signals {
		mark := ""
		if sig.Mock {
			mark = " [MOCK]"
		}
		fmt.Fprintf(&b, "  - %s: %s%s\n", sig.Label, sig.Value, mark)
	}
	fmt.Fprintln(&b, "Routines")
	for _, r := range s.Routines {
		fmt.Fprintf(&b, "  - %02d:%02d  %s (%s)\n", r.Hour, r.Minute, r.Title, r.Window)
	}
	fmt.Fprintln(&b, "Suggestions")
	if len(s.Suggestions) == 0 {
		fmt.Fprintln(&b, "  - none")
	}
	for _, sug := range s.Suggestions {
		fmt.Fprintf(&b, "  - %s — %s [%s] propose=%v reason=%s\n", sug.Title, sug.Body, sug.Source, sug.Propose, sug.Reason)
	}
	return b.String()
}
