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
	w := weather.New(weather.Rain)
	cal := calendar.New()
	al := alarm.New()

	plan, err := umbrella.Execute(w, cal, al, clk)
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

func TestClearSkipsCalendarAndAlarm(t *testing.T) {
	clk, _ := shanghaiFriday()
	w := weather.New(weather.Clear)
	cal := calendar.New()
	al := alarm.New()

	plan, err := umbrella.Execute(w, cal, al, clk)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Weather.IsMock || plan.Weather.Condition != weather.Clear {
		t.Fatalf("unexpected forecast %#v", plan.Weather)
	}
	if plan.CreatedReminders() {
		t.Fatal("clear weather must not create reminders")
	}
	if plan.SkippedReason == "" {
		t.Fatal("expected skip reason")
	}
	if len(cal.Events()) != 0 || len(al.Alarms()) != 0 {
		t.Fatal("stubs should stay empty")
	}
	if len(plan.Steps) != 3 || plan.Steps[1].Status != umbrella.StepSkipped || plan.Steps[2].Status != umbrella.StepSkipped {
		t.Fatalf("steps: %+v", plan.Steps)
	}
}

func TestCloudyAlsoSkips(t *testing.T) {
	clk, _ := shanghaiFriday()
	plan, err := umbrella.Execute(weather.New(weather.Cloudy), calendar.New(), alarm.New(), clk)
	if err != nil {
		t.Fatal(err)
	}
	if plan.CreatedReminders() {
		t.Fatal("cloudy should skip")
	}
}

func TestPlannerRecognizesEnglishAndChinese(t *testing.T) {
	clk, _ := shanghaiFriday()
	w := weather.New(weather.Rain)
	cal := calendar.New()
	al := alarm.New()

	en, err := planner.Plan("bring an umbrella tomorrow 8am", w, cal, al, clk)
	if err != nil {
		t.Fatal(err)
	}
	zh, err := planner.Plan("明天早上八点提醒带伞", w, cal, al, clk)
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
	_, err := planner.Plan("what is 2 plus 2", weather.New(weather.Rain), calendar.New(), alarm.New(), clk)
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := err.(*planner.UnrecognizedGoalError); !ok {
		t.Fatalf("wrong error type: %T %v", err, err)
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
