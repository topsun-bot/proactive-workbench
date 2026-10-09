package today

import (
	"fmt"
	"strings"
	"time"

	"github.com/topsun-bot/proactive-workbench/internal/memory"
	"github.com/topsun-bot/proactive-workbench/internal/tools/weather"
)

type Suggestion struct {
	Title  string
	Body   string
	Kind   string
	Source string
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
	People       []memory.Person
	Preferences  []memory.Preference
	Goals        []memory.Goal
	Tasks        []memory.Task
	Routines     []memory.Routine
	Signals      []Signal
	Suggestions  []Suggestion
	MemorySource string
}

type Input struct {
	Now     time.Time
	Weather weather.Condition
}

func Build(in Input, mem memory.Store) Snapshot {
	if mem.SourceLabel == "" {
		mem = memory.Fixture()
	}
	loc := in.Now.Location()
	if loc == nil {
		loc = time.UTC
	}
	wx := in.Weather
	if wx == "" {
		wx = weather.Clear
	}
	s := Snapshot{
		Greeting:     greeting(in.Now),
		DateLabel:    in.Now.Format("Monday, 2 January 2006"),
		Timezone:     loc.String(),
		WeatherLine:  fmt.Sprintf("%s, %d°C, precip %d%%", wx.DisplayName(), int(wx.TemperatureC()), wx.PrecipPct()),
		WeatherMock:  true,
		SleepLine:    mem.SleepNote,
		People:       mem.People,
		Preferences:  mem.Preferences,
		Goals:        mem.Goals,
		Tasks:        mem.Tasks,
		Routines:     mem.Routines,
		MemorySource: mem.SourceLabel,
	}
	s.Briefing = fmt.Sprintf(
		"%s %s. MOCK weather is %s. %s. Next meeting is standup with Sam at 10:00.",
		s.Greeting, s.DateLabel, s.WeatherLine, mem.SleepNote,
	)
	s.Signals = []Signal{
		{Label: "Sleep", Value: fmt.Sprintf("%.1fh last night", mem.SleepHours), Mock: true},
		{Label: "Weather", Value: s.WeatherLine, Mock: true},
		{Label: "Next meeting", Value: "10:00 standup with Sam", Mock: true},
		{Label: "People nearby", Value: peopleLine(mem.People), Mock: true},
	}
	s.Suggestions = suggestions(mem, wx)
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

func peopleLine(people []memory.Person) string {
	names := make([]string, 0, len(people))
	for _, p := range people {
		names = append(names, p.Name+" ("+p.Relation+")")
	}
	return strings.Join(names, ", ")
}

func suggestions(mem memory.Store, wx weather.Condition) []Suggestion {
	var out []Suggestion
	hasWalk := false
	hasTen := false
	for _, t := range mem.Tasks {
		if t.ID == "walk" {
			hasWalk = true
		}
		if t.Hour == 10 {
			hasTen = true
		}
	}
	if mem.SleepHours > 0 && mem.SleepHours < 6 && hasWalk && hasTen {
		out = append(out, Suggestion{
			Title:  "Move the walk to evening?",
			Body:   "You slept little last night and have a 10am meeting, move the walk to evening?",
			Kind:   "unprompted",
			Source: memory.SourceLabel,
		})
	}
	if wx == weather.Rain {
		out = append(out, Suggestion{
			Title:  "Pack an umbrella",
			Body:   "MOCK forecast is rain. The 07:30 walk and 10:00 commute will be wet.",
			Kind:   "weather",
			Source: weather.SourceLabel,
		})
	}
	return out
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
		fmt.Fprintf(&b, "  - %s — %s [%s]\n", sug.Title, sug.Body, sug.Source)
	}
	return b.String()
}
