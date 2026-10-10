package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestTickParkThenDedupe(t *testing.T) {
	var out bytes.Buffer
	err := run([]string{
		"tick",
		"--weather=clear",
		"--place=home",
		"--activity=idle",
		"--tz=Asia/Shanghai",
		"--now=2026-10-10T15:00:00+08:00",
		"--repeat=2",
	}, &out, &out)
	if err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "FIXTURE weather") {
		t.Fatalf("must label fixture weather\n%s", text)
	}
	if !strings.Contains(text, "visit_park") {
		t.Fatalf("expected park goal\n%s", text)
	}
	if !strings.Contains(text, "Interrupt: YES") {
		t.Fatalf("first tick should interrupt\n%s", text)
	}
	if !strings.Contains(text, "situation unchanged") {
		t.Fatalf("second tick should dedupe\n%s", text)
	}
	yes := strings.Count(text, "Interrupt: YES")
	no := strings.Count(text, "Interrupt: NO")
	if yes != 1 || no != 1 {
		t.Fatalf("want one YES and one NO, got yes=%d no=%d\n%s", yes, no, text)
	}
}

func TestTickRainUmbrella(t *testing.T) {
	var out bytes.Buffer
	err := run([]string{
		"tick",
		"--weather=rain",
		"--now=2026-10-09T13:00:00+08:00",
		"--tz=Asia/Shanghai",
	}, &out, &out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "umbrella_reminder") {
		t.Fatalf("expected umbrella goal\n%s", out.String())
	}
}

func TestUnknownCommand(t *testing.T) {
	if err := run([]string{"explode"}, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestHelp(t *testing.T) {
	var out bytes.Buffer
	if err := run(nil, &out, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "proactivity tick") {
		t.Fatalf("usage:\n%s", out.String())
	}
}

func TestBriefFixture(t *testing.T) {
	var out bytes.Buffer
	err := run([]string{
		"brief",
		"--weather=clear",
		"--now=2026-10-10T15:00:00+08:00",
		"--tz=Asia/Shanghai",
		"--memory-fixture",
	}, &out, &out)
	if err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "FIXTURE") {
		t.Fatalf("brief must label fixtures\n%s", text)
	}
	if !strings.Contains(text, "Morning brief") {
		t.Fatalf("expected brief header\n%s", text)
	}
}

func TestTickJSON(t *testing.T) {
	var out bytes.Buffer
	err := run([]string{
		"tick",
		"--weather=clear",
		"--now=2026-10-10T15:00:00+08:00",
		"--tz=Asia/Shanghai",
		"--json",
	}, &out, &out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"ok": true`) && !strings.Contains(out.String(), `"ok":true`) {
		t.Fatalf("json envelope\n%s", out.String())
	}
	if !strings.Contains(out.String(), `"goalKind": "visit_park"`) && !strings.Contains(out.String(), `"goalKind":"visit_park"`) {
		t.Fatalf("goal\n%s", out.String())
	}
	if !strings.Contains(out.String(), `"propose": true`) && !strings.Contains(out.String(), `"propose":true`) {
		t.Fatalf("propose\n%s", out.String())
	}
	if !strings.Contains(out.String(), `"reason"`) {
		t.Fatalf("reason\n%s", out.String())
	}
}
