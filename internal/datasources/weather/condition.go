package weather

import (
	"fmt"
	"strings"
)

// Condition is a coarse forecast class used by the proactivity core.
type Condition string

const (
	ConditionUnknown     Condition = "unknown"
	ConditionClear       Condition = "clear"
	ConditionCloudy      Condition = "cloudy"
	ConditionRain        Condition = "rain"
	ConditionSnow        Condition = "snow"
	ConditionFog         Condition = "fog"
	ConditionStorm       Condition = "storm"
	ConditionUnavailable Condition = "unavailable"
)

// ParseCondition accepts the fixture/CLI names (clear, rain, cloudy, …).
func ParseCondition(raw string) (Condition, bool) {
	switch Condition(strings.ToLower(strings.TrimSpace(raw))) {
	case ConditionClear, ConditionCloudy, ConditionRain, ConditionSnow, ConditionFog, ConditionStorm, ConditionUnknown, ConditionUnavailable:
		return Condition(strings.ToLower(strings.TrimSpace(raw))), true
	default:
		return ConditionUnknown, false
	}
}

func (c Condition) NeedsUmbrella() bool {
	return c == ConditionRain || c == ConditionStorm
}

// OutdoorOK is the “weather is good” test for a park-style goal.
func (c Condition) OutdoorOK() bool {
	return c == ConditionClear
}

func (c Condition) DisplayName() string {
	switch c {
	case ConditionClear:
		return "Clear"
	case ConditionCloudy:
		return "Cloudy"
	case ConditionRain:
		return "Rain"
	case ConditionSnow:
		return "Snow"
	case ConditionFog:
		return "Fog"
	case ConditionStorm:
		return "Storm"
	case ConditionUnknown:
		return "Unknown"
	case ConditionUnavailable:
		return "Unavailable"
	default:
		return string(c)
	}
}

// ConditionFromWMO maps Open-Meteo / WMO-4677 weather codes.
// Unknown codes stay unknown rather than being guessed as “clear”.
func ConditionFromWMO(code int) Condition {
	switch {
	case code == 0 || code == 1:
		return ConditionClear
	case code == 2 || code == 3:
		return ConditionCloudy
	case code == 45 || code == 48:
		return ConditionFog
	case (code >= 51 && code <= 67) || (code >= 80 && code <= 82):
		return ConditionRain
	case (code >= 71 && code <= 77) || (code >= 85 && code <= 86):
		return ConditionSnow
	case code >= 95 && code <= 99:
		return ConditionStorm
	default:
		return ConditionUnknown
	}
}

func (c Condition) String() string { return string(c) }

func requireCondition(raw string) (Condition, error) {
	c, ok := ParseCondition(raw)
	if !ok {
		return ConditionUnknown, fmt.Errorf("weather: unknown condition %q", raw)
	}
	return c, nil
}
