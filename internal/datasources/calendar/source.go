package calendar

import (
	"context"
	"fmt"
	"time"
)

const FixtureSourceLabel = "FIXTURE calendar — not a live store"

type Event struct {
	UID      string
	Title    string
	Start    time.Time
	End      time.Time
	Notes    string
	Location string
	IsMock   bool
	Source   string
}

type Reminder struct {
	EventUID    string
	Description string
	TriggerAt   time.Time
	Action      string
	IsMock      bool
	Source      string
}

type Window struct {
	From time.Time
	To   time.Time
}

func (w Window) Valid() error {
	if w.From.IsZero() || w.To.IsZero() {
		return fmt.Errorf("calendar: window from/to are required")
	}
	if !w.To.After(w.From) {
		return fmt.Errorf("calendar: window to must be after from")
	}
	return nil
}

func (e Event) Overlaps(w Window) bool {
	end := e.End
	if end.IsZero() {
		end = e.Start.Add(time.Hour)
	}
	return e.Start.Before(w.To) && end.After(w.From)
}

// Source lists events and VALARM reminders. Implementations are read-only.
type Source interface {
	ListEvents(ctx context.Context, w Window) ([]Event, error)
	ListReminders(ctx context.Context, w Window) ([]Reminder, error)
}

func HasUmbrellaEvent(events []Event) bool {
	for _, ev := range events {
		title := ev.Title
		if containsFold(title, "umbrella") || containsFold(title, "带伞") {
			return true
		}
	}
	return false
}

func NextBusy(events []Event, at time.Time, within time.Duration) (Event, bool) {
	limit := at.Add(within)
	var best Event
	found := false
	for _, ev := range events {
		if ev.Start.Before(at) || !ev.Start.Before(limit) {
			continue
		}
		if !found || ev.Start.Before(best.Start) {
			best = ev
			found = true
		}
	}
	return best, found
}

func containsFold(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub ||
		indexFold(s, sub) >= 0)
}
