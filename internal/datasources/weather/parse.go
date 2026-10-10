package weather

import (
	"encoding/json"
	"fmt"
	"time"
)

// FixtureSourceLabel is stamped on every parsed fixture / mock snapshot.
const FixtureSourceLabel = "FIXTURE weather — not a live observation"

// OpenMeteoSourceLabel is stamped on snapshots that came from the HTTP adapter
// (still not a fixture; the bytes may be live or injected).
const OpenMeteoSourceLabel = "open-meteo forecast API"

type openMeteoResponse struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Timezone  string  `json:"timezone"`
	Current   *struct {
		Time          string  `json:"time"`
		Temperature2m float64 `json:"temperature_2m"`
		WeatherCode   int     `json:"weather_code"`
		Precipitation float64 `json:"precipitation"`
	} `json:"current"`
	Hourly *struct {
		Time                     []string  `json:"time"`
		Temperature2m            []float64 `json:"temperature_2m"`
		PrecipitationProbability []int     `json:"precipitation_probability"`
		Precipitation            []float64 `json:"precipitation"`
		WeatherCode              []int     `json:"weather_code"`
	} `json:"hourly"`
}

// ParseOpenMeteoJSON turns an Open-Meteo forecast body into a Snapshot.
// loc is the query location (label is preserved). tz is used when the
// payload omits a parseable timezone.
func ParseOpenMeteoJSON(body []byte, loc Location, tz *time.Location, isMock bool, source string) (Snapshot, error) {
	if len(body) == 0 {
		return Snapshot{}, fmt.Errorf("weather: empty Open-Meteo body")
	}
	if tz == nil {
		tz = time.UTC
	}
	var raw openMeteoResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return Snapshot{}, fmt.Errorf("weather: decode Open-Meteo JSON: %w", err)
	}
	zone := tz
	if raw.Timezone != "" {
		if loaded, err := time.LoadLocation(raw.Timezone); err == nil {
			zone = loaded
		}
	}

	var hourly []HourlyPoint
	if raw.Hourly != nil {
		n := len(raw.Hourly.Time)
		if len(raw.Hourly.Temperature2m) != n ||
			len(raw.Hourly.WeatherCode) != n ||
			len(raw.Hourly.PrecipitationProbability) != n ||
			len(raw.Hourly.Precipitation) != n {
			return Snapshot{}, fmt.Errorf("weather: Open-Meteo hourly arrays have unequal length")
		}
		hourly = make([]HourlyPoint, 0, n)
		for i := 0; i < n; i++ {
			at, err := parseOpenMeteoTime(raw.Hourly.Time[i], zone)
			if err != nil {
				return Snapshot{}, fmt.Errorf("weather: hourly time[%d]: %w", i, err)
			}
			code := raw.Hourly.WeatherCode[i]
			hourly = append(hourly, HourlyPoint{
				At:              at,
				Condition:       ConditionFromWMO(code),
				WMOCode:         code,
				TemperatureC:    raw.Hourly.Temperature2m[i],
				PrecipProbPct:   raw.Hourly.PrecipitationProbability[i],
				PrecipitationMM: raw.Hourly.Precipitation[i],
			})
		}
	}

	snap := Snapshot{
		Location: loc,
		Hourly:   hourly,
		IsMock:   isMock,
		Source:   source,
	}
	if raw.Current != nil && raw.Current.Time != "" {
		at, err := parseOpenMeteoTime(raw.Current.Time, zone)
		if err != nil {
			return Snapshot{}, fmt.Errorf("weather: current time: %w", err)
		}
		snap.ObservedAt = at
		snap.Current = HourlyPoint{
			At:              at,
			Condition:       ConditionFromWMO(raw.Current.WeatherCode),
			WMOCode:         raw.Current.WeatherCode,
			TemperatureC:    raw.Current.Temperature2m,
			PrecipitationMM: raw.Current.Precipitation,
		}
		if p, ok := snap.PointAt(at); ok {
			snap.Current.PrecipProbPct = p.PrecipProbPct
		}
	} else if len(hourly) > 0 {
		snap.Current = hourly[0]
		snap.ObservedAt = hourly[0].At
	}
	return snap, nil
}

func parseOpenMeteoTime(raw string, loc *time.Location) (time.Time, error) {
	if raw == "" {
		return time.Time{}, fmt.Errorf("empty timestamp")
	}
	layouts := []string{
		"2006-01-02T15:04",
		"2006-01-02T15:04:05",
		time.RFC3339,
	}
	var last error
	for _, layout := range layouts {
		t, err := time.ParseInLocation(layout, raw, loc)
		if err == nil {
			return t, nil
		}
		last = err
	}
	return time.Time{}, last
}
