package calendar_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/topsun-bot/proactive-workbench/internal/datasources/calendar"
	"github.com/topsun-bot/proactive-workbench/internal/tool"
)

func shanghai() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		panic(err)
	}
	return loc
}

func fridayWindow() calendar.Window {
	loc := shanghai()
	return calendar.Window{
		From: time.Date(2026, 10, 9, 0, 0, 0, 0, loc),
		To:   time.Date(2026, 10, 10, 0, 0, 0, 0, loc),
	}
}

func TestFixtureICSParsesStandupAndAlarm(t *testing.T) {
	src := calendar.MustFixtureMock()
	events, err := src.ListEvents(context.Background(), fridayWindow())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("friday events: %d", len(events))
	}
	ev := events[0]
	if ev.UID != "fixture-standup-20261009@proactive-workbench" {
		t.Fatalf("uid %s", ev.UID)
	}
	if !strings.Contains(ev.Title, "FIXTURE") {
		t.Fatalf("title must be labeled FIXTURE: %s", ev.Title)
	}
	if !ev.IsMock || ev.Source != calendar.FixtureSourceLabel {
		t.Fatalf("mock labels: %#v", ev)
	}
	wantStart := time.Date(2026, 10, 9, 14, 0, 0, 0, shanghai())
	if !ev.Start.Equal(wantStart) {
		t.Fatalf("start %s want %s", ev.Start, wantStart)
	}

	rems, err := src.ListReminders(context.Background(), calendar.Window{
		From: time.Date(2026, 10, 9, 13, 0, 0, 0, shanghai()),
		To:   time.Date(2026, 10, 9, 14, 0, 0, 0, shanghai()),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rems) != 1 {
		t.Fatalf("reminders: %d", len(rems))
	}
	if !rems[0].TriggerAt.Equal(time.Date(2026, 10, 9, 13, 45, 0, 0, shanghai())) {
		t.Fatalf("trigger %s", rems[0].TriggerAt)
	}
}

func TestFixtureDentistOutsideFriday(t *testing.T) {
	src := calendar.MustFixtureMock()
	events, err := src.ListEvents(context.Background(), fridayWindow())
	if err != nil {
		t.Fatal(err)
	}
	for _, ev := range events {
		if strings.Contains(ev.Title, "Dentist") {
			t.Fatal("dentist is on 2026-10-12, must not appear in Friday window")
		}
	}
}

func TestICSFileAdapter(t *testing.T) {
	dir := t.TempDir()
	srcFile := filepath.Join("fixtures", "sample.ics")
	body, err := os.ReadFile(srcFile)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "copy.ics")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	ics, err := calendar.NewICS(path)
	if err != nil {
		t.Fatal(err)
	}
	ics.Location = shanghai()
	ics.IsFixture = true
	events, err := ics.ListEvents(context.Background(), fridayWindow())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events %d", len(events))
	}
}

func TestICSDirectoryAdapter(t *testing.T) {
	dir := t.TempDir()
	body, err := os.ReadFile(filepath.Join("fixtures", "sample.ics"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.ics"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	ics, err := calendar.NewICS(dir)
	if err != nil {
		t.Fatal(err)
	}
	ics.Location = shanghai()
	events, err := ics.ListEvents(context.Background(), calendar.Window{
		From: time.Date(2026, 10, 1, 0, 0, 0, 0, shanghai()),
		To:   time.Date(2026, 10, 31, 0, 0, 0, 0, shanghai()),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("expected both fixture events, got %d", len(events))
	}
}

func TestCalDAVStubNeverDials(t *testing.T) {
	src, err := calendar.NewCalDAV("https://caldav.example.invalid/calendars/user/", "user")
	if err != nil {
		t.Fatal(err)
	}
	_, err = src.ListEvents(context.Background(), fridayWindow())
	if !errors.Is(err, calendar.ErrCalDAVStub) {
		t.Fatalf("got %v", err)
	}
}

func TestMockWindowValidation(t *testing.T) {
	src := calendar.NewMock()
	_, err := src.ListEvents(context.Background(), calendar.Window{})
	if err == nil {
		t.Fatal("empty window must fail")
	}
}

func TestHasUmbrellaEvent(t *testing.T) {
	events := []calendar.Event{{Title: "Bring an umbrella / 带伞"}}
	if !calendar.HasUmbrellaEvent(events) {
		t.Fatal("expected match")
	}
	if calendar.HasUmbrellaEvent([]calendar.Event{{Title: "Standup"}}) {
		t.Fatal("standup is not an umbrella reminder")
	}
}

func TestPluginListEvents(t *testing.T) {
	wrap := calendar.NewListTool(calendar.MustFixtureMock())
	from := time.Date(2026, 10, 9, 0, 0, 0, 0, shanghai()).Format(time.RFC3339)
	to := time.Date(2026, 10, 10, 0, 0, 0, 0, shanghai()).Format(time.RFC3339)
	res, err := wrap.Handle(tool.Request{Action: "listEvents", Payload: map[string]string{"from": from, "to": to}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Data["count"] != "1" {
		t.Fatalf("count=%q", res.Data["count"])
	}
}

func TestUnfoldedDescription(t *testing.T) {
	raw := []byte("BEGIN:VCALENDAR\nBEGIN:VEVENT\nUID:fold-1\nDTSTART:20261009T140000\nSUMMARY:FIXTURE folded\nDESCRIPTION:line one\n  continued\nEND:VEVENT\nEND:VCALENDAR\n")
	events, _, err := calendar.ParseICS(raw, calendar.FixtureSourceLabel, true, shanghai())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Notes != "line onecontinued" {
		t.Fatalf("notes %#v", events)
	}
}
