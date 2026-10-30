package today

import (
	"context"
	"fmt"
	"strings"
	"time"

	dsweather "github.com/topsun-bot/proactive-workbench/internal/datasources/weather"
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
	Greeting            string
	DateLabel           string
	Timezone            string
	Briefing            string
	WeatherLine         string
	WeatherMock         bool
	WeatherAvailable    bool
	WeatherError        string
	WeatherUserMessage  string
	CalendarAvailable   bool
	CalendarError       string
	CalendarUserMessage string
	SleepLine           string
	People              []Person
	Preferences         []Preference
	Goals               []Goal
	Tasks               []Task
	Routines            []Routine
	Signals             []Signal
	Suggestions         []Suggestion
	MemorySource        string
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
	weatherLine := dsweather.UserFacingUnavailable
	weatherAvail := false
	wxErr := ""
	wxUser := dsweather.UserFacingUnavailable
	calAvail := true
	calErr := ""
	calUser := ""

	var suggestions []Suggestion
	if in.Core != nil {
		res, err := in.Core.Tick(context.Background())
		if err == nil {
			gate := proactivity.FirstGateFrom(res)
			w := proactivity.ResultToSuggestion(res)
			suggestions = []Suggestion{{
				Title:   w.Title,
				Body:    w.Body,
				Kind:    w.Kind,
				Source:  w.Source,
				Propose: gate.Propose,
				Reason:  gate.Reason,
			}}
			if !weatherMock {
				if res.Perception.Weather.Available() {
					p := res.Perception.NowWeather
					weatherLine = fmt.Sprintf("%s, %d°C, precip %d%%", p.Condition.DisplayName(), int(p.TemperatureC), p.PrecipProbPct)
					weatherAvail = true
					wxUser = ""
					wxErr = ""
					weatherMock = res.Perception.Weather.IsMock
				} else {
					weatherLine = dsweather.UserFacingUnavailable
					wxErr = res.Perception.Weather.Error
					wxUser = dsweather.UserFacingUnavailable
				}
			}
			calErr = res.Perception.CalendarError
			calUser = res.Perception.CalendarUserMessage
			calAvail = calErr == ""
		}
	}
	if weatherMock {
		weatherLine = fmt.Sprintf("%s, %d°C, precip %d%%", wx.DisplayName(), int(wx.TemperatureC()), wx.PrecipPct())
		weatherAvail = true
		wxUser = ""
	}
	s := Snapshot{
		Greeting:            greeting(in.Now),
		DateLabel:           in.Now.Format("Monday, 2 January 2006"),
		Timezone:            loc.String(),
		WeatherLine:         weatherLine,
		WeatherMock:         weatherMock,
		WeatherAvailable:    weatherAvail,
		WeatherError:        wxErr,
		WeatherUserMessage:  wxUser,
		CalendarAvailable:   calAvail,
		CalendarError:       calErr,
		CalendarUserMessage: calUser,
		SleepLine:           mem.SleepNote,
		People:              mem.People,
		Preferences:         mem.Preferences,
		Goals:               mem.Goals,
		Tasks:               mem.Tasks,
		Routines:            mem.Routines,
		MemorySource:        mem.SourceLabel,
		Suggestions:         suggestions,
	}
	weatherBrief := s.WeatherLine
	if weatherMock {
		weatherBrief = "MOCK weather is " + s.WeatherLine
	} else if weatherAvail {
		weatherBrief = "Weather " + s.WeatherLine
	} else {
		weatherBrief = "Weather unavailable (" + dsweather.UserFacingUnavailable + ")"
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
