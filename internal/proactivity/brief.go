package proactivity

import (
	"fmt"

	"github.com/topsun-bot/proactive-workbench/internal/memory"
)

// Brief is a generated daily briefing. When built from fixtures it is labeled.
type Brief struct {
	Date               string   `json:"date"`
	IsFixture          bool     `json:"isFixture"`
	Source             string   `json:"source"`
	Weather            string   `json:"weather"`
	Events             []string `json:"events"`
	Memory             []string `json:"memory"`
	Suggestion         string   `json:"suggestion"`
	SourcesAreFixtures bool     `json:"sourcesAreFixtures"`
}

const BriefFixtureSource = "FIXTURE morning brief — weather, calendar, and memory are fixtures"

func BuildBrief(p Perception, snap memory.Snapshot, suggestion Goal) Brief {
	b := Brief{
		Date:               p.At.Format("2006-01-02"),
		IsFixture:          p.Weather.IsMock || p.Situation.IsMock || snap.IsFixture,
		Source:             BriefFixtureSource,
		Events:             []string{},
		Memory:             []string{},
		SourcesAreFixtures: p.Weather.IsMock || p.Situation.IsMock || snap.IsFixture,
	}
	if !b.IsFixture {
		b.Source = "morning brief"
	}
	b.Weather = fmt.Sprintf("%s %.1f°C now; tomorrow 08:00 %s %.1f°C [%s]",
		p.NowWeather.Condition.DisplayName(),
		p.NowWeather.TemperatureC,
		p.TomorrowAM.Condition.DisplayName(),
		p.TomorrowAM.TemperatureC,
		p.Weather.Source,
	)
	if len(p.Events) == 0 {
		b.Events = append(b.Events, "No calendar events in the next 24 hours.")
	} else {
		for _, ev := range p.Events {
			line := fmt.Sprintf("%s %s", ev.Start.Format("15:04"), ev.Title)
			if person, ok := snap.PersonNamedIn(ev.Title + " " + ev.Notes); ok {
				line += " (memory: " + person.Name + ")"
			}
			b.Events = append(b.Events, line)
		}
	}
	if snap.IsFixture {
		b.Memory = append(b.Memory, snap.Source)
	}
	for _, c := range snap.CommitmentsOn(p.At) {
		b.Memory = append(b.Memory, fmt.Sprintf("Commitment today %s: %s", c.When.Format("15:04"), c.Title))
	}
	for _, g := range snap.ActiveGoals() {
		b.Memory = append(b.Memory, "Active goal: "+g.Title)
	}
	for _, person := range snap.People {
		b.Memory = append(b.Memory, "Person: "+person.Name+" ("+person.Relation+")")
	}
	if v, ok := snap.Preference(memory.PrefLikesOutdoors); ok {
		b.Memory = append(b.Memory, "Preference likes_outdoors="+v)
	}
	if len(b.Memory) == 0 {
		b.Memory = append(b.Memory, "No local memory records.")
	}
	if !suggestion.IsNone() {
		b.Suggestion = fmt.Sprintf("%s (score %d): %s", suggestion.Kind, suggestion.Score, suggestion.Reason)
	} else {
		b.Suggestion = "No proactive suggestion this morning."
	}
	return b
}

func FormatBrief(b Brief) string {
	var buf []byte
	w := func(format string, args ...any) {
		buf = append(buf, []byte(fmt.Sprintf(format, args...))...)
	}
	w("Morning brief — %s\n", b.Date)
	w("====================\n")
	w("Source:  %s\n", b.Source)
	w("Weather: %s\n", b.Weather)
	w("\nCalendar\n")
	for _, line := range b.Events {
		w("  - %s\n", line)
	}
	w("\nMemory\n")
	for _, line := range b.Memory {
		w("  - %s\n", line)
	}
	w("\nSuggestion\n")
	w("  %s\n", b.Suggestion)
	return string(buf)
}
