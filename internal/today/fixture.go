package today

// LongTerm is the Today UI fixture: people, preferences, goals, routines.
// It is labeled MOCK and is not read from a live device. Gate-1 propose/reason
// do not come from this store — those are copied from proactivity.Core.
type LongTerm struct {
	SourceLabel string
	SleepHours  float64
	SleepNote   string
	People      []Person
	Preferences []Preference
	Goals       []Goal
	Routines    []Routine
	Tasks       []Task
}

type Person struct {
	Name     string
	Relation string
	Note     string
}

type Preference struct {
	Key   string
	Value string
}

type Goal struct {
	Title   string
	Horizon string
}

type Routine struct {
	ID     string
	Title  string
	Hour   int
	Minute int
	Window string
}

type Task struct {
	ID     string
	Title  string
	Hour   int
	Minute int
	Kind   string
}

const FixtureSourceLabel = "MOCK long-term memory (not live data)"

// Fixture is the labeled today.ai-style memory used by the Snapshot briefing.
func Fixture() LongTerm {
	return LongTerm{
		SourceLabel: FixtureSourceLabel,
		SleepHours:  4.5,
		SleepNote:   "MOCK sleep last night: 4.5h (poor)",
		People: []Person{
			{Name: "Alex", Relation: "roommate", Note: "Usually joins the 07:30 walk"},
			{Name: "Sam", Relation: "manager", Note: "Runs the 10:00 standup"},
		},
		Preferences: []Preference{
			{Key: "walk", Value: "morning 07:30"},
			{Key: "quiet_hours", Value: "22:00–08:00"},
			{Key: "commute", Value: "hates wet morning commutes"},
		},
		Goals: []Goal{
			{Title: "Run a 10k in November", Horizon: "2026-11"},
		},
		Routines: []Routine{
			{ID: "walk", Title: "Morning walk", Hour: 7, Minute: 30, Window: "morning"},
			{ID: "standup", Title: "Team standup", Hour: 10, Minute: 0, Window: "morning"},
			{ID: "stretch", Title: "Evening stretch", Hour: 20, Minute: 0, Window: "evening"},
		},
		Tasks: []Task{
			{ID: "walk", Title: "Morning walk", Hour: 7, Minute: 30, Kind: "routine"},
			{ID: "standup", Title: "Standup with Sam", Hour: 10, Minute: 0, Kind: "meeting"},
			{ID: "review", Title: "Review umbrella demo PR", Hour: 14, Minute: 0, Kind: "focus"},
		},
	}
}
