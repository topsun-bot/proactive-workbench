package umbrella

import (
	"fmt"
	"time"

	"github.com/topsun-bot/proactive-workbench/internal/clock"
	"github.com/topsun-bot/proactive-workbench/internal/scope"
	"github.com/topsun-bot/proactive-workbench/internal/tool"
	"github.com/topsun-bot/proactive-workbench/internal/tools/alarm"
	"github.com/topsun-bot/proactive-workbench/internal/tools/calendar"
	"github.com/topsun-bot/proactive-workbench/internal/tools/weather"
)

const (
	IntentID       = "umbrella-tomorrow-8am"
	ReminderHour   = 8
	ReminderMinute = 0
	EventTitle     = "Bring an umbrella / 带伞"
)

type Outcome string

const (
	OutcomeRain               Outcome = "rain"
	OutcomeNoRain             Outcome = "no_rain"
	OutcomeWeatherUnavailable Outcome = "weather_unavailable"
)

type StepStatus string

const (
	StepDone    StepStatus = "done"
	StepSkipped StepStatus = "skipped"
)

type Step struct {
	ToolID string
	Title  string
	Detail string
	Status StepStatus
}

// Plan is the cross-tool result: weather + calendar + alarm.
type Plan struct {
	IntentID         string
	ReminderAt       time.Time
	Weather          weather.Forecast
	Outcome          Outcome
	ReminderMessage  string
	RainThresholdPct int
	CalendarEvent    *calendar.Event
	Alarm            *alarm.Record
	SkippedReason    string
	Steps            []Step
}

func (p Plan) CreatedReminders() bool {
	return p.CalendarEvent != nil && p.Alarm != nil
}

// Execute talks to weather, calendar, and alarm only through the plugin
// registry. Per docs/PRD.md the reminder is always created: rain, no-rain,
// and weather-unavailable all set calendar+alarm with different copy.
func Execute(reg *tool.Registry, clk clock.Clock) (Plan, error) {
	w, err := requireTool(reg, weather.ToolID)
	if err != nil {
		return Plan{}, err
	}
	cal, err := requireTool(reg, calendar.ToolID)
	if err != nil {
		return Plan{}, err
	}
	al, err := requireTool(reg, alarm.ToolID)
	if err != nil {
		return Plan{}, err
	}

	when, err := clk.NextMorning(ReminderHour, ReminderMinute)
	if err != nil {
		return Plan{}, err
	}

	threshold := scope.Find().RainThresholdPct
	if threshold <= 0 {
		threshold = scope.DefaultRainThresholdPct
	}

	forecast, weatherStep, outcome, message, err := resolveWeather(w, when, threshold)
	if err != nil {
		return Plan{}, err
	}
	plan := Plan{
		IntentID:         IntentID,
		ReminderAt:       when,
		Weather:          forecast,
		Outcome:          outcome,
		ReminderMessage:  message,
		RainThresholdPct: threshold,
		Steps:            []Step{weatherStep},
	}

	whenISO := when.Format(time.RFC3339)
	calRes, err := cal.Handle(tool.Request{
		Action: "createEvent",
		Payload: map[string]string{
			"title":         EventTitle,
			"start":         whenISO,
			"notes":         message,
			"createdByFlow": IntentID,
		},
	})
	if err != nil {
		return Plan{}, fmt.Errorf("create calendar event: %w", err)
	}
	if err := requireSuccess(calRes, "calendar"); err != nil {
		return Plan{}, err
	}
	ev, err := calendar.EventFromResult(calRes)
	if err != nil {
		return Plan{}, err
	}

	alarmRes, err := al.Handle(tool.Request{
		Action: "createAlarm",
		Payload: map[string]string{
			"label":         EventTitle,
			"fireDate":      whenISO,
			"createdByFlow": IntentID,
		},
	})
	if err != nil || !alarmRes.Success {
		if rbErr := rollbackCalendar(cal, ev.ID); rbErr != nil {
			return Plan{}, fmt.Errorf("create alarm failed after calendar persist (%v); rollback also failed: %w", alarmFailure(err, alarmRes), rbErr)
		}
		if err != nil {
			return Plan{}, fmt.Errorf("create alarm: %w (calendar event rolled back)", err)
		}
		return Plan{}, fmt.Errorf("create alarm: plugin reported failure: %s (calendar event rolled back)", alarmRes.Summary)
	}
	rec, err := alarm.RecordFromResult(alarmRes)
	if err != nil {
		if rbErr := rollbackCalendar(cal, ev.ID); rbErr != nil {
			return Plan{}, fmt.Errorf("alarm result invalid (%v); rollback also failed: %w", err, rbErr)
		}
		return Plan{}, fmt.Errorf("alarm result invalid: %w (calendar event rolled back)", err)
	}

	plan.CalendarEvent = &ev
	plan.Alarm = &rec
	plan.Steps = append(plan.Steps,
		Step{
			ToolID: calendar.ToolID,
			Title:  "Create calendar event",
			Detail: fmt.Sprintf("%q at %s", ev.Title, ev.Start.In(clk.Location).Format("2006-01-02 15:04 MST")),
			Status: StepDone,
		},
		Step{
			ToolID: alarm.ToolID,
			Title:  "Set alarm",
			Detail: fmt.Sprintf("%q at %s", rec.Label, rec.FireAt.In(clk.Location).Format("2006-01-02 15:04 MST")),
			Status: StepDone,
		},
	)
	return plan, nil
}

func resolveWeather(w tool.Tool, when time.Time, threshold int) (weather.Forecast, Step, Outcome, string, error) {
	forecastRes, err := w.Handle(tool.Request{
		Action:  "forecast",
		Payload: map[string]string{"date": when.Format(time.RFC3339)},
	})
	if err != nil || !forecastRes.Success {
		msg := "记得带伞（天气暂时查不到）。"
		detail := "Weather query failed; reminder still set with fallback copy. No fabricated values."
		if forecastRes.Summary != "" {
			detail = forecastRes.Summary + " — reminder still set, no fabricated values."
		} else if err != nil {
			detail = err.Error() + " — reminder still set, no fabricated values."
		}
		return weather.Forecast{Available: false, IsMock: true, SourceLabel: weather.SourceLabel},
			Step{ToolID: weather.ToolID, Title: "Check weather", Detail: detail, Status: StepDone},
			OutcomeWeatherUnavailable, msg, nil
	}
	forecast, err := weather.ForecastFromResult(forecastRes, when)
	if err != nil {
		return weather.Forecast{}, Step{}, "", "", err
	}
	if forecast.PrecipPct >= threshold {
		msg := fmt.Sprintf("今天可能下雨（降水概率 %d%%），记得带伞。", forecast.PrecipPct)
		return forecast,
			Step{ToolID: weather.ToolID, Title: "Check weather", Detail: forecastRes.Summary, Status: StepDone},
			OutcomeRain, msg, nil
	}
	msg := fmt.Sprintf("今天降水概率 %d%%，可能用不上伞，带不带你定。", forecast.PrecipPct)
	return forecast,
		Step{ToolID: weather.ToolID, Title: "Check weather", Detail: forecastRes.Summary, Status: StepDone},
		OutcomeNoRain, msg, nil
}

func rollbackCalendar(cal tool.Tool, id string) error {
	res, err := cal.Handle(tool.Request{
		Action:  "deleteEvent",
		Payload: map[string]string{"id": id},
	})
	if err != nil {
		return err
	}
	if !res.Success {
		return fmt.Errorf("deleteEvent: %s", res.Summary)
	}
	return nil
}

func requireSuccess(res tool.Result, what string) error {
	if res.Success {
		return nil
	}
	if res.Summary != "" {
		return fmt.Errorf("%s: plugin reported failure: %s", what, res.Summary)
	}
	return fmt.Errorf("%s: plugin reported failure", what)
}

func alarmFailure(err error, res tool.Result) error {
	if err != nil {
		return err
	}
	if res.Summary != "" {
		return fmt.Errorf("%s", res.Summary)
	}
	return fmt.Errorf("plugin reported failure")
}

func requireTool(reg *tool.Registry, id string) (tool.Tool, error) {
	t, ok := reg.Get(id)
	if !ok {
		return nil, fmt.Errorf("required tool %q is not registered", id)
	}
	return t, nil
}
