package situation

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestSnapshotLabeledMOCK(t *testing.T) {
	m := MustMock(PlaceHome, ActivityIdle)
	snap, err := m.Snapshot(context.Background(), time.Date(2026, 10, 10, 15, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !snap.IsMock {
		t.Fatal("fixture situation must set IsMock")
	}
	if !strings.Contains(snap.Source, "MOCK") {
		t.Fatalf("location must be labeled MOCK, got %q", snap.Source)
	}
	if strings.Contains(strings.ToLower(snap.Source), "geoclue") && !strings.Contains(snap.Source, "not a live GPS or GeoClue") {
		t.Fatalf("label must deny GeoClue, got %q", snap.Source)
	}
}
