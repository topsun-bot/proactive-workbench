package memory

import "time"

// FixtureLocation is Asia/Shanghai so fixture timestamps match the CLI default.
func fixtureTZ() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.UTC
	}
	return loc
}

// FixtureSnapshot is a labeled demo store. It is not a live user profile and
// does not copy characters or routines from today.ai.
func FixtureSnapshot() Snapshot {
	loc := fixtureTZ()
	sat := time.Date(2026, 10, 10, 17, 0, 0, 0, loc)
	return Snapshot{
		Version:   SchemaVersion,
		UpdatedAt: time.Time{},
		IsFixture: true,
		Source:    FixtureSource,
		Commitments: []Commitment{
			{
				ID:     "fixture-weekly-review",
				Title:  "FIXTURE weekly review",
				When:   sat,
				Notes:  "Synthetic commitment for tests (Saturday 17:00 Asia/Shanghai).",
				Source: FixtureSource,
			},
		},
		People: []Person{
			{
				ID:       "fixture-colleague",
				Name:     "FIXTURE colleague",
				Relation: "coworker",
				Notes:    "Name is a label, not a real person. Used to test calendar/title matching.",
				Source:   FixtureSource,
			},
		},
		Preferences: []Preference{
			{Key: PrefQuietStart, Value: "22", Source: FixtureSource},
			{Key: PrefQuietEnd, Value: "8", Source: FixtureSource},
			{Key: PrefLikesOutdoors, Value: "true", Source: FixtureSource},
			{Key: PrefMorningHour, Value: "8", Source: FixtureSource},
			{Key: PrefMorningMinute, Value: "0", Source: FixtureSource},
		},
		Goals: []LongTermGoal{
			{
				ID:     "fixture-outdoors",
				Title:  "Spend more time outdoors",
				Status: "active",
				Notes:  "Synthetic long-term goal for park-relevance tests.",
				Source: FixtureSource,
			},
		},
	}
}
