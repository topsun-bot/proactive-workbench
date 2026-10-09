package weather

import (
	"strconv"
	"sync"
	"time"

	"github.com/topsun-bot/proactive-workbench/internal/tool"
)

// Condition is a mock forecast. This tool never calls a live weather API.
type Condition string

const (
	Rain   Condition = "rain"
	Clear  Condition = "clear"
	Cloudy Condition = "cloudy"
)

func (c Condition) NeedsUmbrella() bool {
	return c == Rain
}

func (c Condition) DisplayName() string {
	switch c {
	case Rain:
		return "Rain"
	case Clear:
		return "Clear"
	case Cloudy:
		return "Cloudy"
	default:
		return string(c)
	}
}

func (c Condition) TemperatureC() float64 {
	switch c {
	case Rain:
		return 16
	case Clear:
		return 22
	case Cloudy:
		return 18
	default:
		return 0
	}
}

func ParseCondition(raw string) (Condition, bool) {
	switch Condition(raw) {
	case Rain, Clear, Cloudy:
		return Condition(raw), true
	default:
		return "", false
	}
}

const (
	// SourceLabel is shown in CLI output and stored on every forecast.
	SourceLabel = "MOCK weather (not live data)"
	ToolID      = "weather"
)

// Forecast is always mock in this skeleton.
type Forecast struct {
	Condition     Condition
	TemperatureC  float64
	LocationLabel string
	ValidFor      time.Time
	IsMock        bool
	SourceLabel   string
}

func (f Forecast) Summary() string {
	return SourceLabel + ": " + f.Condition.DisplayName() +
		", " + strconv.Itoa(int(f.TemperatureC)) + "°C in " + f.LocationLabel
}

// Tool is an in-memory mock weather plugin.
type Tool struct {
	mu            sync.Mutex
	scenario      Condition
	locationLabel string
}

func New(scenario Condition) *Tool {
	if scenario == "" {
		scenario = Rain
	}
	return &Tool{scenario: scenario, locationLabel: "Local"}
}

func (t *Tool) Descriptor() tool.Descriptor {
	return tool.Descriptor{
		ID:          ToolID,
		DisplayName: "Weather",
		Summary:     "MOCK weather forecasts — not live data. Swap this plugin for a real provider later.",
	}
}

func (t *Tool) SetScenario(c Condition) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.scenario = c
}

func (t *Tool) Scenario() Condition {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.scenario
}

func (t *Tool) Forecast(forTime time.Time) Forecast {
	t.mu.Lock()
	defer t.mu.Unlock()
	return Forecast{
		Condition:     t.scenario,
		TemperatureC:  t.scenario.TemperatureC(),
		LocationLabel: t.locationLabel,
		ValidFor:      forTime,
		IsMock:        true,
		SourceLabel:   SourceLabel,
	}
}

func (t *Tool) Handle(req tool.Request) (tool.Result, error) {
	switch req.Action {
	case "forecast":
		when := time.Now()
		if raw, ok := req.Payload["date"]; ok && raw != "" {
			parsed, err := time.Parse(time.RFC3339, raw)
			if err != nil {
				return tool.Result{}, tool.InvalidPayload("date must be RFC3339")
			}
			when = parsed
		}
		f := t.Forecast(when)
		return tool.Result{
			Success: true,
			Summary: f.Summary(),
			Data: map[string]string{
				"condition":    string(f.Condition),
				"temperatureC": strconv.Itoa(int(f.TemperatureC)),
				"isMock":       "true",
				"sourceLabel":  f.SourceLabel,
				"location":     f.LocationLabel,
			},
		}, nil
	case "setScenario":
		raw := ""
		if req.Payload != nil {
			raw = req.Payload["condition"]
		}
		next, ok := ParseCondition(raw)
		if !ok {
			return tool.Result{}, tool.InvalidPayload("condition must be rain, clear, or cloudy")
		}
		t.SetScenario(next)
		return tool.Result{Success: true, Summary: "Mock scenario set to " + next.DisplayName()}, nil
	default:
		return tool.Result{}, tool.Unsupported(req.Action)
	}
}
