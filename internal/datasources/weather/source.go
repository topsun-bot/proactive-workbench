package weather

import (
	"context"
	"fmt"
	"time"
)

// Location is a forecast query, not a live GPS fix unless the caller says so.
type Location struct {
	Latitude  float64
	Longitude float64
	Label     string
}

// DefaultLocation is Shanghai Changning District, used when the user has
// not set lat/lon in config. This is not a device location and is never
// obtained from GeoClue or CoreLocation.
//
// Coordinates 31.2205°N, 121.4248°E are the Changning NPC Committee point
// published on Wikipedia “Changning, Shanghai”
// (https://en.wikipedia.org/wiki/Changning,_Shanghai), DMS 31°13′14″N
// 121°25′29″E. The district government is at 1320 Yuyuan Rd in the same
// district. Four-decimal rounding matches the Open-Meteo query format.
var DefaultLocation = Location{
	Latitude:  31.2205,
	Longitude: 121.4248,
	Label:     "Shanghai Changning District (Wikipedia NPC Committee point; not a live GPS fix)",
}

// HourlyPoint is one hour from a forecast or fixture series.
type HourlyPoint struct {
	At              time.Time
	Condition       Condition
	WMOCode         int
	TemperatureC    float64
	PrecipProbPct   int
	PrecipitationMM float64
}

// Snapshot is a labeled weather read. IsMock / Source must stay honest:
// fixtures say they are fixtures; Open-Meteo says it is Open-Meteo.
// On a live fetch failure, Condition is unavailable, Error is set, and
// TemperatureC / precip are left at zero — never filled with fixture numbers.
type Snapshot struct {
	ObservedAt time.Time
	Location   Location
	Current    HourlyPoint
	Hourly     []HourlyPoint
	IsMock     bool
	Source     string
	// Error is an English log/API reason when the live query failed.
	Error string
}

// Available is false when the live query failed or the condition is unavailable.
func (s Snapshot) Available() bool {
	return s.Error == "" && s.Current.Condition != ConditionUnavailable
}

func (s Snapshot) Summary() string {
	return fmt.Sprintf("%s %.1f°C [%s]",
		s.Current.Condition.DisplayName(),
		s.Current.TemperatureC,
		s.Source,
	)
}

// PointAt returns the hourly row closest to at (same location timezone).
func (s Snapshot) PointAt(at time.Time) (HourlyPoint, bool) {
	if len(s.Hourly) == 0 {
		return HourlyPoint{}, false
	}
	best := s.Hourly[0]
	bestDelta := absDuration(at.Sub(best.At))
	for _, p := range s.Hourly[1:] {
		d := absDuration(at.Sub(p.At))
		if d < bestDelta {
			best = p
			bestDelta = d
		}
	}
	return best, true
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

// OutdoorComfort is the park-suggestion weather gate.
func OutdoorComfort(p HourlyPoint) bool {
	if !p.Condition.OutdoorOK() {
		return false
	}
	if p.TemperatureC < 12 || p.TemperatureC > 28 {
		return false
	}
	if p.PrecipProbPct > 30 || p.PrecipitationMM > 0 {
		return false
	}
	return true
}

// Source reads weather. Implementations must not hide mock vs live.
type Source interface {
	Snapshot(ctx context.Context, loc Location, at time.Time) (Snapshot, error)
}

func validLocation(loc Location) error {
	if loc.Latitude < -90 || loc.Latitude > 90 {
		return fmt.Errorf("weather: latitude %v out of range", loc.Latitude)
	}
	if loc.Longitude < -180 || loc.Longitude > 180 {
		return fmt.Errorf("weather: longitude %v out of range", loc.Longitude)
	}
	return nil
}
