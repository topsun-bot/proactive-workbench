package weather

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Named fixture IDs. These select bundled JSON, not live readings.
const (
	FixtureClear  = "clear"
	FixtureRain   = "rain"
	FixtureCloudy = "cloudy"
)

// Mock is a Weather Source backed by labeled fixture JSON.
// Snapshot never performs I/O beyond the already-embedded bytes.
type Mock struct {
	mu       sync.Mutex
	fixture  string
	location Location
	tz       *time.Location
}

func NewMock(fixture string) (*Mock, error) {
	if fixture == "" {
		fixture = FixtureClear
	}
	if _, err := requireCondition(fixture); err != nil && fixture != FixtureClear && fixture != FixtureRain && fixture != FixtureCloudy {
		return nil, err
	}
	switch fixture {
	case FixtureClear, FixtureRain, FixtureCloudy:
	default:
		return nil, fmt.Errorf("weather: unknown fixture %q (use clear, rain, cloudy)", fixture)
	}
	return &Mock{
		fixture:  fixture,
		location: DefaultLocation,
		tz:       time.UTC,
	}, nil
}

func MustMock(fixture string) *Mock {
	m, err := NewMock(fixture)
	if err != nil {
		panic(err)
	}
	return m
}

func (m *Mock) SetLocation(loc Location) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.location = loc
}

func (m *Mock) SetLocationTZ(loc *time.Location) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if loc != nil {
		m.tz = loc
	}
}

func (m *Mock) FixtureName() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.fixture
}

func (m *Mock) Snapshot(ctx context.Context, loc Location, at time.Time) (Snapshot, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return Snapshot{}, err
		}
	}
	m.mu.Lock()
	name := m.fixture
	fallback := m.location
	zone := m.tz
	m.mu.Unlock()

	if loc.Label == "" && loc.Latitude == 0 && loc.Longitude == 0 {
		loc = fallback
	}
	if err := validLocation(loc); err != nil {
		return Snapshot{}, err
	}
	body, err := fixtureJSON(name)
	if err != nil {
		return Snapshot{}, err
	}
	snap, err := ParseOpenMeteoJSON(body, loc, zone, true, FixtureSourceLabel)
	if err != nil {
		return Snapshot{}, err
	}
	if !at.IsZero() {
		if p, ok := snap.PointAt(at); ok {
			snap.Current = p
			snap.ObservedAt = at
		}
	}
	return snap, nil
}

func fixtureJSON(name string) ([]byte, error) {
	switch name {
	case FixtureClear:
		return fixtureClearJSON, nil
	case FixtureRain:
		return fixtureRainJSON, nil
	case FixtureCloudy:
		return fixtureCloudyJSON, nil
	default:
		return nil, fmt.Errorf("weather: no fixture named %q", name)
	}
}
