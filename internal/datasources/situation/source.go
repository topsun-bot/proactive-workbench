package situation

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Place is a coarse location class. It is not a GPS coordinate.
type Place string

const (
	PlaceUnknown Place = "unknown"
	PlaceHome    Place = "home"
	PlaceWork    Place = "work"
	PlaceAway    Place = "away"
)

// Activity is a coarse occupancy class (“bored” ≈ idle).
type Activity string

const (
	ActivityUnknown Activity = "unknown"
	ActivityIdle    Activity = "idle"
	ActivityBusy    Activity = "busy"
)

// MockSourceLabel is stamped on every situation snapshot. Location is
// fixture-only: no GeoClue, no CoreLocation, no live GPS.
const MockSourceLabel = "MOCK location — fixture-only, not a live GPS or GeoClue fix"

// FixtureSourceLabel is an alias of MockSourceLabel (kept for existing callers).
const FixtureSourceLabel = MockSourceLabel

type Snapshot struct {
	At       time.Time
	Place    Place
	Activity Activity
	IsMock   bool
	Source   string
}

type Source interface {
	Snapshot(ctx context.Context, at time.Time) (Snapshot, error)
}

func ParsePlace(raw string) (Place, bool) {
	switch Place(strings.ToLower(strings.TrimSpace(raw))) {
	case PlaceHome, PlaceWork, PlaceAway, PlaceUnknown:
		return Place(strings.ToLower(strings.TrimSpace(raw))), true
	default:
		return PlaceUnknown, false
	}
}

func ParseActivity(raw string) (Activity, bool) {
	switch Activity(strings.ToLower(strings.TrimSpace(raw))) {
	case ActivityIdle, ActivityBusy, ActivityUnknown:
		return Activity(strings.ToLower(strings.TrimSpace(raw))), true
	default:
		return ActivityUnknown, false
	}
}

// Mock returns a caller-chosen place and activity. Default: home + idle
// (the owner’s “at home, bored” example).
type Mock struct {
	mu       sync.Mutex
	place    Place
	activity Activity
}

func NewMock(place Place, activity Activity) (*Mock, error) {
	if place == "" {
		place = PlaceHome
	}
	if activity == "" {
		activity = ActivityIdle
	}
	switch place {
	case PlaceHome, PlaceWork, PlaceAway, PlaceUnknown:
	default:
		return nil, fmt.Errorf("situation: unknown place %q", place)
	}
	switch activity {
	case ActivityIdle, ActivityBusy, ActivityUnknown:
	default:
		return nil, fmt.Errorf("situation: unknown activity %q", activity)
	}
	return &Mock{place: place, activity: activity}, nil
}

func MustMock(place Place, activity Activity) *Mock {
	m, err := NewMock(place, activity)
	if err != nil {
		panic(err)
	}
	return m
}

func (m *Mock) Snapshot(ctx context.Context, at time.Time) (Snapshot, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return Snapshot{}, err
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return Snapshot{
		At:       at,
		Place:    m.place,
		Activity: m.activity,
		IsMock:   true,
		Source:   FixtureSourceLabel,
	}, nil
}
