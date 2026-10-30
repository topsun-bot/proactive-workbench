package calendar_test

import (
	"context"
	"testing"

	"github.com/topsun-bot/proactive-workbench/internal/datasources/calendar"
)

func TestUnconfiguredDoesNotInventEvents(t *testing.T) {
	src := calendar.NewUnconfigured()
	events, err := src.ListEvents(context.Background(), fridayWindow())
	if err == nil {
		t.Fatal("expected unconfigured error")
	}
	if events != nil {
		t.Fatalf("invented events %#v", events)
	}
	if calendar.UserMessageForError(err) != calendar.UserFacingUnconfigured {
		t.Fatalf("msg %q", calendar.UserMessageForError(err))
	}
}
