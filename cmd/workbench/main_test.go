package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/topsun-bot/proactive-workbench/internal/tools/weather"
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
		"今天可能下雨",
		"降水概率 80%",
		"记得带伞",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("output missing %q\n%s", want, text)
		}
	}
}

func TestDemoClearStillCreatesReminders(t *testing.T) {
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
	text := out.String()
	for _, want := range []string{
		"Reminders created",
		"Outcome:  no_rain",
		"今天降水概率 5%",
		"带不带你定",
		"MOCK weather (not live data)",
		"2026-10-10 08:00",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("output missing %q\n%s", want, text)
		}
	}
	if strings.Contains(text, "Reminders skipped") {
		t.Fatal("PRD: clear weather must not skip reminders")
	}
}

func TestDemoUnavailableStillCreatesReminders(t *testing.T) {
	var out bytes.Buffer
	err := run([]string{
		"demo",
		"--weather=unavailable",
		"--tz=Asia/Shanghai",
		"--now=2026-10-09T13:00:00+08:00",
	}, &out, &out)
	if err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{
		"Reminders created",
		"Outcome:  weather_unavailable",
		"记得带伞（天气暂时查不到）。",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("output missing %q\n%s", want, text)
		}
	}
	if strings.Contains(text, "降水概率") && strings.Contains(text, "天气暂时查不到") {
		// fallback message must not invent a probability
		if strings.Contains(text, "今天降水概率") {
			t.Fatalf("unavailable path fabricated precip\n%s", text)
		}
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

func TestTodayBriefing(t *testing.T) {
	var out bytes.Buffer
	err := run([]string{
		"today",
		"--weather=clear",
		"--tz=Asia/Shanghai",
		"--now=2026-10-10T07:15:00+08:00",
	}, &out, &out)
	if err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{
		"Good morning",
		"MOCK",
		"Morning walk",
		"Team standup",
		"propose=",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q\n%s", want, text)
		}
	}
}

func TestUnknownCommand(t *testing.T) {
	if err := run([]string{"explode"}, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestDemoHelpIsSuccess(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"demo", "--help"}, &out, &out); err != nil {
		t.Fatalf("demo --help should exit 0: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "Usage:") {
		t.Fatalf("expected usage text\n%s", out.String())
	}
	if strings.Contains(out.String(), "Reminders created") {
		t.Fatal("help must not run the demo")
	}
}

func TestPlanHelpIsSuccess(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"plan", "bring an umbrella tomorrow 8am", "--help"}, &out, &out); err != nil {
		t.Fatalf("plan ... --help should exit 0: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "Usage:") {
		t.Fatalf("expected usage text\n%s", out.String())
	}
}

func TestPlanRejectsEightPM(t *testing.T) {
	var out bytes.Buffer
	err := run([]string{
		"plan", "bring an umbrella tomorrow at eight pm",
		"--weather=rain",
		"--now=2026-10-09T13:00:00+08:00",
		"--tz=Asia/Shanghai",
	}, &out, &out)
	if err == nil {
		t.Fatalf("expected unrecognized goal, got\n%s", out.String())
	}
}

func TestServeDefaultsToLiveClockAndNoMockWeather(t *testing.T) {
	t.Setenv("PW_DEBUG_FIXTURE", "")
	t.Setenv("PW_DEBUG_NOW", "")
	t.Setenv("PW_DEBUG_WEATHER", "")
	opts, err := parseServeArgs(nil, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if opts.addr != "127.0.0.1:8741" {
		t.Fatalf("addr = %q", opts.addr)
	}
	if !opts.now.IsZero() {
		t.Fatalf("default now must be live, got %s", opts.now)
	}
	if opts.weatherSet || opts.weather != "" {
		t.Fatalf("default must not inject MOCK weather, got %+v", opts)
	}
	if opts.debugOn {
		t.Fatal("debug fixture must be off by default")
	}
}

func TestServeDebugFixtureFlag(t *testing.T) {
	t.Setenv("PW_DEBUG_FIXTURE", "")
	t.Setenv("PW_DEBUG_NOW", "")
	t.Setenv("PW_DEBUG_WEATHER", "")
	opts, err := parseServeArgs([]string{"--debug-fixture"}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.debugOn {
		t.Fatal("expected debug on")
	}
	if opts.now.Format(time.RFC3339) != "2026-10-10T07:15:00+08:00" {
		t.Fatalf("fixture now = %s", opts.now.Format(time.RFC3339))
	}
	if opts.weather != weather.Clear || !opts.weatherSet {
		t.Fatalf("fixture weather = %q", opts.weather)
	}
}

func TestServeDebugEnv(t *testing.T) {
	t.Setenv("PW_DEBUG_FIXTURE", "1")
	t.Setenv("PW_DEBUG_NOW", "2026-10-11T09:00:00+08:00")
	t.Setenv("PW_DEBUG_WEATHER", "rain")
	opts, err := parseServeArgs(nil, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if opts.now.Format(time.RFC3339) != "2026-10-11T09:00:00+08:00" {
		t.Fatalf("env now = %s", opts.now.Format(time.RFC3339))
	}
	if opts.weather != weather.Rain {
		t.Fatalf("env weather = %q", opts.weather)
	}
}

func TestServeHelpIsSuccess(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"serve", "--help"}, &out, &out); err != nil {
		t.Fatalf("serve --help should exit 0: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "127.0.0.1:8741") {
		t.Fatalf("help should mention default port\n%s", out.String())
	}
	if !strings.Contains(out.String(), "PW_DEBUG_FIXTURE") {
		t.Fatalf("help should document debug env\n%s", out.String())
	}
}

func TestPlanRejectsUnsupportedUmbrellaTime(t *testing.T) {
	var out bytes.Buffer
	err := run([]string{
		"plan", "remind me to bring an umbrella today at 5pm",
		"--weather=rain",
		"--now=2026-10-09T13:00:00+08:00",
		"--tz=Asia/Shanghai",
	}, &out, &out)
	if err == nil {
		t.Fatalf("expected unrecognized goal, got\n%s", out.String())
	}
}

func TestTodayUnavailableDoesNotInventClear(t *testing.T) {
	var out bytes.Buffer
	err := run([]string{
		"today",
		"--weather=unavailable",
		"--tz=Asia/Shanghai",
		"--now=2026-10-10T07:15:00+08:00",
	}, &out, &out)
	if err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "Weather unavailable (no MOCK scenario)") {
		t.Fatalf("expected honest unavailable weather line, got:\n%s", text)
	}
	if strings.Contains(text, "Clear · 22°C") {
		t.Fatalf("today --weather=unavailable must not fabricate Clear · 22°C:\n%s", text)
	}
}

