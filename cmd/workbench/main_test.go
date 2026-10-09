package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestDemoRainShowsMockAndCreatesReminders(t *testing.T) {
	var out bytes.Buffer
	err := run([]string{
		"demo",
		"--weather=rain",
		"--tz=Asia/Shanghai",
		"--now=2026-10-09T13:00:00+08:00",
	}, &out, &out)
	if err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{
		"明天早上八点提醒带伞",
		"MOCK weather (not live data)",
		"isMock:   true",
		"Rain",
		"Reminders created",
		"calendar:",
		"alarm:",
		"2026-10-10 08:00",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("output missing %q\n%s", want, text)
		}
	}
}

func TestDemoClearSkipsReminders(t *testing.T) {
	var out bytes.Buffer
	err := run([]string{
		"demo",
		"--weather=clear",
		"--tz=Asia/Shanghai",
		"--now=2026-10-09T13:00:00+08:00",
	}, &out, &out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Reminders skipped") {
		t.Fatalf("expected skip path\n%s", out.String())
	}
	if strings.Contains(out.String(), "Reminders created") {
		t.Fatal("clear should not create reminders")
	}
}

func TestPlanEnglishPhrase(t *testing.T) {
	var out bytes.Buffer
	err := run([]string{
		"plan", "bring an umbrella tomorrow 8am",
		"--weather=rain",
		"--now=2026-10-09T13:00:00+08:00",
		"--tz=Asia/Shanghai",
	}, &out, &out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Reminders created") {
		t.Fatalf("expected reminders\n%s", out.String())
	}
}

func TestUnknownCommand(t *testing.T) {
	if err := run([]string{"explode"}, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
		t.Fatal("expected error")
	}
}
