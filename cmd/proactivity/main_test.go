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
