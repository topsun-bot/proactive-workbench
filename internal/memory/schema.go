package memory

import (
	"fmt"
	"strings"
	"time"
)

const (
	SchemaVersion = 1
	FixtureSource = "FIXTURE memory — not a live user store"
)

// Well-known preference keys (values are strings).
const (
	PrefQuietStart    = "quiet_hours_start"
	PrefQuietEnd      = "quiet_hours_end"
	PrefLikesOutdoors = "likes_outdoors"
	PrefMorningHour   = "morning_brief_hour"
	PrefMorningMinute = "morning_brief_minute"
)

type Commitment struct {
	ID     string    `json:"id"`
	Title  string    `json:"title"`
	When   time.Time `json:"when"`
	Notes  string    `json:"notes"`
	Source string    `json:"source"`
}

type Person struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Relation string `json:"relation"`
	Notes    string `json:"notes"`
	Source   string `json:"source"`
}

type Preference struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Source string `json:"source"`
}

type LongTermGoal struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"` // active | paused | done
	Notes  string `json:"notes"`
	Source string `json:"source"`
}

// Snapshot is the on-disk document. All writes replace the whole file.
type Snapshot struct {
	Version     int            `json:"version"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	IsFixture   bool           `json:"isFixture"`
	Source      string         `json:"source"`
	Commitments []Commitment   `json:"commitments"`
	People      []Person       `json:"people"`
	Preferences []Preference   `json:"preferences"`
	Goals       []LongTermGoal `json:"goals"`
}

func Empty() Snapshot {
	return Snapshot{
		Version:     SchemaVersion,
		UpdatedAt:   time.Time{},
		IsFixture:   false,
		Source:      "local on-disk memory",
		Commitments: []Commitment{},
		People:      []Person{},
		Preferences: []Preference{},
		Goals:       []LongTermGoal{},
	}
}

func (s Snapshot) Preference(key string) (string, bool) {
	for _, p := range s.Preferences {
		if p.Key == key {
			return p.Value, true
		}
	}
	return "", false
}

func (s Snapshot) LikesOutdoors() bool {
	v, ok := s.Preference(PrefLikesOutdoors)
	if !ok {
		return true // unspecified: do not invent a dislike
	}
	return v == "true" || v == "yes" || v == "1"
}

// QuietHours returns preference-overridden quiet hours. ok is false when
// neither preference is set; callers should keep Policy defaults.
func (s Snapshot) QuietHours() (start, end int, ok bool) {
	start, end = -1, -1
	if v, found := s.Preference(PrefQuietStart); found {
		if h, err := parseHour(v); err == nil {
			start = h
			ok = true
		}
	}
	if v, found := s.Preference(PrefQuietEnd); found {
		if h, err := parseHour(v); err == nil {
			end = h
			ok = true
		}
	}
	return start, end, ok
}

// MorningBriefClock returns the configured local hour:minute for the daily
// brief. Unset preferences fall back to 08:00 (today.ai’s Morning Brief time,
// used here only as the default clock, not as a claim about their product).
func (s Snapshot) MorningBriefClock() (hour, minute int) {
	hour, minute = 8, 0
	if v, ok := s.Preference(PrefMorningHour); ok {
		if h, err := parseHour(v); err == nil {
			hour = h
		}
	}
	if v, ok := s.Preference(PrefMorningMinute); ok {
		if m, err := parseMinute(v); err == nil {
			minute = m
		}
	}
	return hour, minute
}

func (s *Snapshot) validate() error {
	if s.Version != SchemaVersion {
		return fmt.Errorf("memory: unsupported schema version %d", s.Version)
	}
	for _, c := range s.Commitments {
		if strings.TrimSpace(c.ID) == "" || strings.TrimSpace(c.Title) == "" {
			return fmt.Errorf("memory: commitment id and title are required")
		}
	}
	for _, p := range s.People {
		if strings.TrimSpace(p.ID) == "" || strings.TrimSpace(p.Name) == "" {
			return fmt.Errorf("memory: person id and name are required")
		}
	}
	for _, p := range s.Preferences {
		if strings.TrimSpace(p.Key) == "" {
			return fmt.Errorf("memory: preference key is required")
		}
	}
	for _, g := range s.Goals {
		if strings.TrimSpace(g.ID) == "" || strings.TrimSpace(g.Title) == "" {
			return fmt.Errorf("memory: goal id and title are required")
		}
		switch g.Status {
		case "active", "paused", "done":
		default:
			return fmt.Errorf("memory: goal status must be active, paused, or done")
		}
	}
	return nil
}

func normalizeGoalStatus(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "active":
		return "active"
	case "paused":
		return "paused"
	case "done":
		return "done"
	default:
		return ""
	}
}

func parseHour(raw string) (int, error) {
	var n int
	_, err := fmt.Sscanf(strings.TrimSpace(raw), "%d", &n)
	if err != nil || n < 0 || n > 23 {
		return 0, fmt.Errorf("memory: hour %q is not 0–23", raw)
	}
	return n, nil
}

func parseMinute(raw string) (int, error) {
	var n int
	_, err := fmt.Sscanf(strings.TrimSpace(raw), "%d", &n)
	if err != nil || n < 0 || n > 59 {
		return 0, fmt.Errorf("memory: minute %q is not 0–59", raw)
	}
	return n, nil
}
