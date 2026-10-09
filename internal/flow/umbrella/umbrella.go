package umbrella

import (
	"fmt"
	"time"

	"github.com/topsun-bot/proactive-workbench/internal/clock"
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

// Plan is the cross-tool result: weather + optional calendar + alarm.
type Plan struct {
	IntentID      string
	ReminderAt    time.Time
	Weather       weather.Forecast
	CalendarEvent *calendar.Event
	Alarm         *alarm.Record
	SkippedReason string
	Steps         []Step
}

func (p Plan) CreatedReminders() bool {
	return p.CalendarEvent != nil && p.Alarm != nil
}

// Execute talks to weather, calendar, and alarm only through the plugin
// registry. A replacement registered under the same tool ID is used as-is.
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

	forecastRes, err := w.Handle(tool.Request{
		Action:  "forecast",
		Payload: map[string]string{"date": when.Format(time.RFC3339)},
	})
	if err != nil {
		return Plan{}, fmt.Errorf("weather forecast: %w", err)
	}
	forecast, err := weather.ForecastFromResult(forecastRes, when)
	if err != nil {
		return Plan{}, err
	}

	plan := Plan{
		IntentID:   IntentID,
		ReminderAt: when,
		Weather:    forecast,
		Steps: []Step{{
			ToolID: weather.ToolID,
			Title:  "Check weather",
			Detail: forecastRes.Summary,
			Status: StepDone,
		}},
	}

	if !forecast.Condition.NeedsUmbrella() {
		reason := "No rain in the weather forecast — calendar event and alarm were not created."
		plan.SkippedReason = reason
		plan.Steps = append(plan.Steps,
			Step{ToolID: calendar.ToolID, Title: "Create calendar event", Detail: "Skipped. " + reason, Status: StepSkipped},
			Step{ToolID: alarm.ToolID, Title: "Set alarm", Detail: "Skipped. " + reason, Status: StepSkipped},
		)
		return plan, nil
	}

	notes := fmt.Sprintf(
		"Pack an umbrella. Forecast is %s (%s, %d°C).",
		forecast.SourceLabel,
		forecast.Condition.DisplayName(),
		int(forecast.TemperatureC),
	)
	whenISO := when.Format(time.RFC3339)

	calRes, err := cal.Handle(tool.Request{
		Action: "createEvent",
		Payload: map[string]string{
			"title":         EventTitle,
			"start":         whenISO,
			"notes":         notes,
			"createdByFlow": IntentID,
		},
	})
	if err != nil {
		return Plan{}, fmt.Errorf("create calendar event: %w", err)
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
	if err != nil {
		return Plan{}, fmt.Errorf("create alarm: %w", err)
	}
	rec, err := alarm.RecordFromResult(alarmRes)
	if err != nil {
		return Plan{}, err
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

func requireTool(reg *tool.Registry, id string) (tool.Tool, error) {
	t, ok := reg.Get(id)
	if !ok {
		return nil, fmt.Errorf("required tool %q is not registered", id)
	}
	return t, nil
}
