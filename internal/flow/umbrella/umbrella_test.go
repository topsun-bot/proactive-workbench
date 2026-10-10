package umbrella_test

import (
	"strings"
	"testing"
	"time"

	"github.com/topsun-bot/proactive-workbench/internal/clock"
	"github.com/topsun-bot/proactive-workbench/internal/flow/umbrella"
	"github.com/topsun-bot/proactive-workbench/internal/intent"
	"github.com/topsun-bot/proactive-workbench/internal/planner"
	"github.com/topsun-bot/proactive-workbench/internal/tool"
	"github.com/topsun-bot/proactive-workbench/internal/tools/alarm"
	"github.com/topsun-bot/proactive-workbench/internal/tools/calendar"
	"github.com/topsun-bot/proactive-workbench/internal/tools/weather"
)

func shanghaiFriday() (clock.Clock, time.Time) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		panic(err)
	}
	// Friday 9 Oct 2026 13:00 CST → tomorrow 08:00 is Sat 10 Oct 2026.
	now := time.Date(2026, 10, 9, 13, 0, 0, 0, loc)
	return clock.Fixed(now, loc), now
}

func session(cond weather.Condition) (*tool.Registry, *calendar.Tool, *alarm.Tool) {
	w := weather.New(cond)
	cal := calendar.New()
	al := alarm.New()
	reg := tool.NewRegistry()
	reg.Register(w)
	reg.Register(cal)
	reg.Register(al)
	return reg, cal, al
}

func TestNextMorningIsTomorrow8AMShanghai(t *testing.T) {
	clk, _ := shanghaiFriday()
	got, err := clk.NextMorning(8, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 10, 10, 8, 0, 0, 0, clk.Location)
	if !got.Equal(want) {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestRainCreatesCalendarAndAlarm(t *testing.T) {
	clk, _ := shanghaiFriday()
	reg, cal, al := session(weather.Rain)

	plan, err := umbrella.Execute(reg, clk)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Weather.IsMock {
		t.Fatal("forecast must be labeled mock")
	}
	if plan.Weather.SourceLabel != weather.SourceLabel {
		t.Fatalf("source label: %q", plan.Weather.SourceLabel)
	}
	if plan.Weather.Condition != weather.Rain {
		t.Fatalf("condition: %s", plan.Weather.Condition)
	}
	if !plan.CreatedReminders() {
		t.Fatal("expected calendar + alarm")
	}
	if plan.Outcome != umbrella.OutcomeRain {
		t.Fatalf("outcome %s", plan.Outcome)
	}
	if !strings.Contains(plan.ReminderMessage, "记得带伞") || !strings.Contains(plan.ReminderMessage, "80%") {
		t.Fatalf("message: %s", plan.ReminderMessage)
	}
	if plan.SkippedReason != "" {
		t.Fatalf("unexpected skip: %s", plan.SkippedReason)
	}
	if plan.CalendarEvent.Title != umbrella.EventTitle {
		t.Fatalf("title: %s", plan.CalendarEvent.Title)
	}
	if plan.CalendarEvent.CreatedByFlow != umbrella.IntentID {
		t.Fatalf("flow: %s", plan.CalendarEvent.CreatedByFlow)
	}
	if !plan.CalendarEvent.Start.Equal(plan.Alarm.FireAt) || !plan.CalendarEvent.Start.Equal(plan.ReminderAt) {
		t.Fatal("calendar, alarm, and reminder timestamps must match")
	}
	if len(cal.Events()) != 1 || len(al.Alarms()) != 1 {
		t.Fatalf("stored counts calendar=%d alarm=%d", len(cal.Events()), len(al.Alarms()))
	}
	if len(plan.Steps) != 3 {
		t.Fatalf("steps: %d", len(plan.Steps))
	}
	for i, step := range plan.Steps {
		if step.Status != umbrella.StepDone {
			t.Fatalf("step %d status %s", i, step.Status)
		}
	}
}

func TestClearStillCreatesReminders(t *testing.T) {
	clk, _ := shanghaiFriday()
	reg, cal, al := session(weather.Clear)

	plan, err := umbrella.Execute(reg, clk)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Weather.IsMock || plan.Weather.Condition != weather.Clear {
		t.Fatalf("unexpected forecast %#v", plan.Weather)
	}
	if !plan.CreatedReminders() {
		t.Fatal("PRD: clear weather must still create reminders")
	}
	if plan.Outcome != umbrella.OutcomeNoRain {
		t.Fatalf("outcome %s", plan.Outcome)
	}
	if !strings.Contains(plan.ReminderMessage, "带不带你定") {
		t.Fatalf("message: %s", plan.ReminderMessage)
	}
	if len(cal.Events()) != 1 || len(al.Alarms()) != 1 {
		t.Fatal("stubs should hold the reminder")
	}
}

func TestCloudyStillCreatesReminders(t *testing.T) {
	clk, _ := shanghaiFriday()
	reg, _, _ := session(weather.Cloudy)
	plan, err := umbrella.Execute(reg, clk)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.CreatedReminders() || plan.Outcome != umbrella.OutcomeNoRain {
		t.Fatalf("cloudy should still remind: %#v", plan)
	}
}

func TestUnavailableWeatherStillCreatesReminders(t *testing.T) {
	clk, _ := shanghaiFriday()
	reg, cal, al := session(weather.Unavailable)
	plan, err := umbrella.Execute(reg, clk)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.CreatedReminders() {
		t.Fatal("PRD: weather failure must still create reminders")
	}
	if plan.Outcome != umbrella.OutcomeWeatherUnavailable {
		t.Fatalf("outcome %s", plan.Outcome)
	}
	if plan.ReminderMessage != "记得带伞（天气暂时查不到）。" {
		t.Fatalf("message: %s", plan.ReminderMessage)
	}
	if strings.Contains(plan.ReminderMessage, "%") || strings.Contains(plan.Steps[0].Detail, "0°C") {
		t.Fatalf("must not fabricate weather numbers: %#v", plan)
	}
	if len(cal.Events()) != 1 || len(al.Alarms()) != 1 {
		t.Fatal("expected persisted reminder")
	}
}

func TestPlannerRecognizesEnglishAndChinese(t *testing.T) {
	clk, _ := shanghaiFriday()
	reg, cal, al := session(weather.Rain)

	en, err := planner.Plan("bring an umbrella tomorrow 8am", reg, clk)
	if err != nil {
		t.Fatal(err)
	}
	zh, err := planner.Plan("明天早上八点提醒带伞", reg, clk)
	if err != nil {
		t.Fatal(err)
	}
	if !en.CreatedReminders() || !zh.CreatedReminders() {
		t.Fatal("both phrases should create reminders")
	}
	if len(cal.Events()) != 2 || len(al.Alarms()) != 2 {
		t.Fatalf("expected two of each, got cal=%d al=%d", len(cal.Events()), len(al.Alarms()))
	}
}

func TestPlannerRejectsUnknownGoal(t *testing.T) {
	clk, _ := shanghaiFriday()
	reg, _, _ := session(weather.Rain)
	_, err := planner.Plan("what is 2 plus 2", reg, clk)
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := err.(*planner.UnrecognizedGoalError); !ok {
		t.Fatalf("wrong error type: %T %v", err, err)
	}
}

func TestPlannerRejectsUmbrellaPhraseWithUnsupportedTime(t *testing.T) {
	clk, _ := shanghaiFriday()
	reg, cal, al := session(weather.Rain)
	_, err := planner.Plan("remind me to bring an umbrella today at 5pm", reg, clk)
	if err == nil {
		t.Fatal("expected unsupported-time umbrella phrase to be rejected")
	}
	if _, ok := err.(*planner.UnrecognizedGoalError); !ok {
		t.Fatalf("wrong error type: %T %v", err, err)
	}
	if len(cal.Events()) != 0 || len(al.Alarms()) != 0 {
		t.Fatal("rejected phrase must not create reminders")
	}
}

func TestIntentRecognizer(t *testing.T) {
	cases := []struct {
		in   string
		kind intent.Kind
	}{
		{"Bring an umbrella tomorrow 8am", intent.UmbrellaReminder},
		{"明天早上八点提醒带伞", intent.UmbrellaReminder},
		{"open calculator", intent.Unknown},
		{"remind me to bring an umbrella today at 5pm", intent.Unknown},
		{"bring an umbrella tomorrow at eight pm", intent.Unknown},
		{"明天十八点提醒带伞", intent.Unknown},
		{"明天带伞", intent.Unknown},
		{"带伞", intent.Unknown},
	}
	for _, tc := range cases {
		got := intent.Recognize(tc.in)
		if got.Kind != tc.kind {
			t.Fatalf("%q: got %v want %v", tc.in, got.Kind, tc.kind)
		}
		if tc.kind == intent.UmbrellaReminder && (got.Hour != 8 || got.Minute != 0) {
			t.Fatalf("%q: time %d:%d", tc.in, got.Hour, got.Minute)
		}
	}
}

func TestWeatherPluginHandleMarksMock(t *testing.T) {
	w := weather.New(weather.Rain)
	res, err := w.Handle(tool.Request{Action: "forecast"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Data["isMock"] != "true" {
		t.Fatalf("isMock=%q", res.Data["isMock"])
	}
	if res.Data["sourceLabel"] != weather.SourceLabel {
		t.Fatalf("source=%q", res.Data["sourceLabel"])
	}
	if !strings.Contains(res.Summary, "MOCK") {
		t.Fatalf("summary should mention MOCK: %s", res.Summary)
	}
}

func TestCalendarCreateEventDefaultsCreatedByFlowBeforePersist(t *testing.T) {
	start := time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC)
	cal := calendar.New()
	res, err := cal.Handle(tool.Request{
		Action:  "createEvent",
		Payload: map[string]string{"title": "Standup", "start": start.Format(time.RFC3339)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Data["createdByFlow"] != "plugin" {
		t.Fatalf("result createdByFlow=%q", res.Data["createdByFlow"])
	}
	stored := cal.Events()
	if len(stored) != 1 || stored[0].CreatedByFlow != "plugin" {
		t.Fatalf("persisted event %#v", stored)
	}
}

func TestCalendarAndAlarmPluginInterface(t *testing.T) {
	start := time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC)
	iso := start.Format(time.RFC3339)
	cal := calendar.New()
	al := alarm.New()

	ev, err := cal.Handle(tool.Request{Action: "createEvent", Payload: map[string]string{"title": "Standup", "start": iso}})
	if err != nil || !ev.Success {
		t.Fatalf("calendar: %v %#v", err, ev)
	}
	alr, err := al.Handle(tool.Request{Action: "createAlarm", Payload: map[string]string{"label": "Standup", "fireDate": iso}})
	if err != nil || !alr.Success {
		t.Fatalf("alarm: %v %#v", err, alr)
	}
	if len(cal.Events()) != 1 || len(al.Alarms()) != 1 {
		t.Fatal("plugin handle did not persist")
	}
}

// replacementWeather is a different concrete type registered as "weather".
type replacementWeather struct{}

func (replacementWeather) Descriptor() tool.Descriptor {
	return tool.Descriptor{
		ID:          weather.ToolID,
		DisplayName: "Replacement weather",
		Summary:     "Alternate plugin used to prove the planner talks to the registry",
	}
}

func (replacementWeather) Handle(req tool.Request) (tool.Result, error) {
	if req.Action != "forecast" {
		return tool.Result{}, tool.Unsupported(req.Action)
	}
	return tool.Result{
		Success: true,
		Summary: "replacement: Rain, 12°C in Lab",
		Data: map[string]string{
			"condition":    "rain",
			"temperatureC": "12",
			"precipPct":    "80",
			"isMock":       "true",
			"sourceLabel":  "replacement weather plugin",
			"location":     "Lab",
		},
	}, nil
}

func TestUmbrellaFlowUsesRegisteredWeatherPlugin(t *testing.T) {
	clk, _ := shanghaiFriday()
	cal := calendar.New()
	al := alarm.New()
	reg := tool.NewRegistry()
	reg.Register(replacementWeather{})
	reg.Register(cal)
	reg.Register(al)

	plan, err := umbrella.Execute(reg, clk)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Weather.LocationLabel != "Lab" || plan.Weather.TemperatureC != 12 {
		t.Fatalf("did not use replacement plugin: %#v", plan.Weather)
	}
	if !plan.CreatedReminders() {
		t.Fatal("replacement rain plugin should create reminders")
	}
	if len(cal.Events()) != 1 {
		t.Fatal("calendar should have been invoked through the registry")
	}
}

func TestExecuteRequiresRegisteredTools(t *testing.T) {
	clk, _ := shanghaiFriday()
	_, err := umbrella.Execute(tool.NewRegistry(), clk)
	if err == nil {
		t.Fatal("expected missing-tool error")
	}
}

type unsuccessfulWeather struct {
	condition string
}

func (unsuccessfulWeather) Descriptor() tool.Descriptor {
	return tool.Descriptor{ID: weather.ToolID, DisplayName: "Unsuccessful weather"}
}

func (u unsuccessfulWeather) Handle(req tool.Request) (tool.Result, error) {
	if req.Action != "forecast" {
		return tool.Result{}, tool.Unsupported(req.Action)
	}
	return tool.Result{
		Success: false,
		Summary: "upstream timeout",
		Data: map[string]string{
			"condition":    u.condition,
			"temperatureC": "16",
		},
	}, nil
}

func TestUnsuccessfulWeatherResultIsNotTreatedAsSuccess(t *testing.T) {
	clk, _ := shanghaiFriday()
	reg := tool.NewRegistry()
	reg.Register(unsuccessfulWeather{condition: "rain"})
	reg.Register(calendar.New())
	reg.Register(alarm.New())
	plan, err := umbrella.Execute(reg, clk)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Outcome != umbrella.OutcomeWeatherUnavailable {
		t.Fatalf("Success=false rain payload must not be treated as rain, got %s", plan.Outcome)
	}
	if !plan.CreatedReminders() {
		t.Fatal("PRD: still remind when weather fails")
	}
}

type badTempWeather struct{}

func (badTempWeather) Descriptor() tool.Descriptor {
	return tool.Descriptor{ID: weather.ToolID, DisplayName: "Bad temp"}
}

func (badTempWeather) Handle(req tool.Request) (tool.Result, error) {
	return tool.Result{
		Success: true,
		Summary: "bad temp",
		Data: map[string]string{
			"condition":    "clear",
			"temperatureC": "warm",
		},
	}, nil
}

func TestMalformedTemperatureIsRejected(t *testing.T) {
	clk, _ := shanghaiFriday()
	reg := tool.NewRegistry()
	reg.Register(badTempWeather{})
	reg.Register(calendar.New())
	reg.Register(alarm.New())
	_, err := umbrella.Execute(reg, clk)
	if err == nil {
		t.Fatal("expected invalid temperatureC to fail")
	}
}

type failingAlarm struct{}

func (failingAlarm) Descriptor() tool.Descriptor {
	return tool.Descriptor{ID: alarm.ToolID, DisplayName: "Failing alarm"}
}

func (failingAlarm) Handle(req tool.Request) (tool.Result, error) {
	return tool.Result{Success: false, Summary: "alarm backend down"}, nil
}

func TestAlarmFailureRollsBackCalendar(t *testing.T) {
	clk, _ := shanghaiFriday()
	cal := calendar.New()
	reg := tool.NewRegistry()
	reg.Register(weather.New(weather.Rain))
	reg.Register(cal)
	reg.Register(failingAlarm{})
	_, err := umbrella.Execute(reg, clk)
	if err == nil {
		t.Fatal("expected alarm failure")
	}
	if !strings.Contains(err.Error(), "rolled back") {
		t.Fatalf("error should mention rollback: %v", err)
	}
	if len(cal.Events()) != 0 {
		t.Fatalf("calendar event should be rolled back, still have %d", len(cal.Events()))
	}
}
