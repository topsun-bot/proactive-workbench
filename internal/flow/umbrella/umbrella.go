package umbrella

import (
	"fmt"
	"time"

	"github.com/topsun-bot/proactive-workbench/internal/clock"
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

// Execute: check mock weather; if rain, create calendar event and alarm.
func Execute(w *weather.Tool, cal *calendar.Tool, al *alarm.Tool, clk clock.Clock) (Plan, error) {
	when, err := clk.NextMorning(ReminderHour, ReminderMinute)
	if err != nil {
		return Plan{}, err
	}
	forecast := w.Forecast(when)

	plan := Plan{
		IntentID:   IntentID,
		ReminderAt: when,
		Weather:    forecast,
		Steps: []Step{{
			ToolID: weather.ToolID,
			Title:  "Check weather",
			Detail: forecast.Summary(),
			Status: StepDone,
		}},
	}

	if !forecast.Condition.NeedsUmbrella() {
		reason := "No rain in the mock forecast — calendar event and alarm were not created."
		plan.SkippedReason = reason
		plan.Steps = append(plan.Steps,
			Step{ToolID: calendar.ToolID, Title: "Create calendar event", Detail: "Skipped. " + reason, Status: StepSkipped},
			Step{ToolID: alarm.ToolID, Title: "Set alarm", Detail: "Skipped. " + reason, Status: StepSkipped},
		)
		return plan, nil
	}

	notes := fmt.Sprintf(
		"Pack an umbrella. Forecast is mock data (%s, %d°C).",
		forecast.Condition.DisplayName(),
		int(forecast.TemperatureC),
	)
	ev := cal.Create(EventTitle, when, notes, IntentID)
	rec := al.Create(EventTitle, when, IntentID)
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
