package memory_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/topsun-bot/proactive-workbench/internal/memory"
)

func TestFileCRUDRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.json")
	store, err := memory.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}

	empty, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if empty.IsFixture || len(empty.People) != 0 {
		t.Fatalf("missing file must load empty, got %#v", empty)
	}

	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	when := time.Date(2026, 10, 10, 17, 0, 0, 0, loc)

	_, err = store.Update(func(s *memory.Snapshot) error {
		if err := s.UpsertCommitment(memory.Commitment{
			ID: "c1", Title: "Review notes", When: when, Source: "test",
		}); err != nil {
			return err
		}
		if err := s.UpsertPerson(memory.Person{
			ID: "p1", Name: "Alex", Relation: "coworker", Source: "test",
		}); err != nil {
			return err
		}
		if err := s.UpsertPreference(memory.Preference{
			Key: memory.PrefLikesOutdoors, Value: "false", Source: "test",
		}); err != nil {
			return err
		}
		return s.UpsertGoal(memory.LongTermGoal{
			ID: "g1", Title: "Read more", Status: "active", Source: "test",
		})
	})
	if err != nil {
		t.Fatal(err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Version != memory.SchemaVersion {
		t.Fatalf("version %d", loaded.Version)
	}
	if !loaded.UpdatedAt.After(time.Time{}) {
		t.Fatal("UpdatedAt must be set on save")
	}
	c, ok := loaded.CommitmentByID("c1")
	if !ok || c.Title != "Review notes" {
		t.Fatalf("commitment %#v", c)
	}
	if loaded.LikesOutdoors() {
		t.Fatal("likes_outdoors was set false")
	}
	if _, ok := loaded.GoalByID("g1"); !ok {
		t.Fatal("goal missing")
	}

	_, err = store.Update(func(s *memory.Snapshot) error {
		if !s.DeleteCommitment("c1") {
			t.Fatal("delete commitment")
		}
		if !s.DeletePerson("p1") {
			t.Fatal("delete person")
		}
		if !s.DeletePreference(memory.PrefLikesOutdoors) {
			t.Fatal("delete pref")
		}
		if !s.DeleteGoal("g1") {
			t.Fatal("delete goal")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	cleared, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cleared.Commitments)+len(cleared.People)+len(cleared.Preferences)+len(cleared.Goals) != 0 {
		t.Fatalf("expected empty after deletes: %#v", cleared)
	}
	if !cleared.LikesOutdoors() {
		t.Fatal("unspecified likes_outdoors defaults to true")
	}
}

func TestRejectsInvalidRecords(t *testing.T) {
	s := memory.Empty()
	if err := s.UpsertCommitment(memory.Commitment{ID: "x"}); err == nil {
		t.Fatal("commitment needs title")
	}
	if err := s.UpsertPerson(memory.Person{ID: "x"}); err == nil {
		t.Fatal("person needs name")
	}
	if err := s.UpsertGoal(memory.LongTermGoal{ID: "g", Title: "t", Status: "maybe"}); err == nil {
		t.Fatal("bad status")
	}
	if err := s.UpsertPreference(memory.Preference{}); err == nil {
		t.Fatal("pref needs key")
	}
}

func TestUnsupportedSchemaVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.json")
	if err := os.WriteFile(path, []byte(`{"version":99,"commitments":[],"people":[],"preferences":[],"goals":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := memory.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Load()
	if err == nil || !strings.Contains(err.Error(), "unsupported schema version") {
		t.Fatalf("got %v", err)
	}
}

func TestFixtureIsLabeledAndSeedOnce(t *testing.T) {
	fix := memory.FixtureSnapshot()
	if !fix.IsFixture || fix.Source != memory.FixtureSource {
		t.Fatalf("fixture must be labeled: %#v", fix)
	}
	if strings.Contains(strings.ToLower(fix.People[0].Name), "maya") {
		t.Fatal("do not copy today.ai demo characters into fixtures")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "memory.json")
	store, err := memory.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.SeedFixture()
	if err != nil {
		t.Fatal(err)
	}
	if !first.IsFixture || len(first.Commitments) == 0 {
		t.Fatalf("seed: %#v", first)
	}
	_, err = store.Update(func(s *memory.Snapshot) error {
		return s.UpsertPreference(memory.Preference{Key: "note", Value: "user-written", Source: "test"})
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.SeedFixture()
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := second.Preference("note"); v != "user-written" {
		t.Fatal("SeedFixture must not overwrite a non-empty store")
	}
}

func TestMorningAndQuietHelpers(t *testing.T) {
	s := memory.Empty()
	h, m := s.MorningBriefClock()
	if h != 8 || m != 0 {
		t.Fatalf("default brief clock %d:%d", h, m)
	}
	if _, _, ok := s.QuietHours(); ok {
		t.Fatal("empty snapshot has no quiet override")
	}
	_ = s.UpsertPreference(memory.Preference{Key: memory.PrefMorningHour, Value: "7"})
	_ = s.UpsertPreference(memory.Preference{Key: memory.PrefMorningMinute, Value: "30"})
	_ = s.UpsertPreference(memory.Preference{Key: memory.PrefQuietStart, Value: "21"})
	_ = s.UpsertPreference(memory.Preference{Key: memory.PrefQuietEnd, Value: "9"})
	h, m = s.MorningBriefClock()
	if h != 7 || m != 30 {
		t.Fatalf("brief clock %d:%d", h, m)
	}
	start, end, ok := s.QuietHours()
	if !ok || start != 21 || end != 9 {
		t.Fatalf("quiet %d %d ok=%v", start, end, ok)
	}
}

func TestOpenFileRequiresJSON(t *testing.T) {
	if _, err := memory.OpenFile(""); err == nil {
		t.Fatal("empty path")
	}
	if _, err := memory.OpenFile(filepath.Join(t.TempDir(), "noext")); err == nil {
		t.Fatal("extension required")
	}
}
