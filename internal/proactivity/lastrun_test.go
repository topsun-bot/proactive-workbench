package proactivity

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/topsun-bot/proactive-workbench/internal/clock"
	"github.com/topsun-bot/proactive-workbench/internal/memory"
)

func TestLoadLastRunMissingOrCorrupt(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "nope.json")
	if got := loadLastRun(missing); len(got) != 0 {
		t.Fatalf("missing file should be empty, got %#v", got)
	}
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := loadLastRun(bad); len(got) != 0 {
		t.Fatalf("corrupt file should be empty, got %#v", got)
	}
}

func TestSaveLastRunAtomicReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "routines-last-run.json")
	at := time.Date(2026, 10, 10, 8, 5, 0, 0, time.UTC)
	if err := saveLastRun(path, map[RoutineKind]time.Time{RoutineMorningBrief: at}); err != nil {
		t.Fatal(err)
	}
	got := loadLastRun(path)
	if !got[RoutineMorningBrief].Equal(at) {
		t.Fatalf("reload %#v", got)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("temp file should be gone after rename")
	}
}

func TestPersistentSchedulerSurvivesRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "routines-last-run.json")
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 10, 8, 5, 0, 0, loc)
	clk := clock.Fixed(at, loc)
	s1 := NewPersistentScheduler(clk, time.Minute, path)
	s1.MarkRan(RoutineMorningBrief, at)

	s2 := NewPersistentScheduler(clk, time.Minute, path)
	snap := memory.FixtureSnapshot()
	for _, st := range s2.Status(snap) {
		if st.Kind == RoutineMorningBrief && st.Due {
			t.Fatal("morning_brief must not re-fire after same-day reload")
		}
	}
}

func TestNewSchedulerDoesNotWriteHome(t *testing.T) {
	s := NewScheduler(clock.Fixed(time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC), time.UTC), time.Minute)
	if s.PersistPath() != "" {
		t.Fatalf("in-process scheduler must have empty persist path, got %q", s.PersistPath())
	}
	s.MarkRan(RoutineMorningBrief, time.Now())
}
