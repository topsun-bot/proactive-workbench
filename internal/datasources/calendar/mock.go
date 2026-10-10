package calendar

import (
	"context"
	"sync"
)

// Mock is an in-memory calendar. Seed it with fixtures or test events.
type Mock struct {
	mu        sync.Mutex
	events    []Event
	reminders []Reminder
}

func NewMock() *Mock {
	return &Mock{}
}

// NewFixtureMock loads the bundled sample.ics (labeled FIXTURE, not real appointments).
func NewFixtureMock() (*Mock, error) {
	events, reminders, err := ParseICS(sampleICS, FixtureSourceLabel, true, nil)
	if err != nil {
		return nil, err
	}
	return &Mock{events: events, reminders: reminders}, nil
}

func MustFixtureMock() *Mock {
	m, err := NewFixtureMock()
	if err != nil {
		panic(err)
	}
	return m
}

func (m *Mock) Add(ev Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ev.IsMock = true
	if ev.Source == "" {
		ev.Source = FixtureSourceLabel
	}
	m.events = append(m.events, ev)
}

func (m *Mock) AddReminder(r Reminder) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r.IsMock = true
	if r.Source == "" {
		r.Source = FixtureSourceLabel
	}
	m.reminders = append(m.reminders, r)
}

func (m *Mock) ListEvents(ctx context.Context, w Window) ([]Event, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	if err := w.Valid(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Event
	for _, ev := range m.events {
		if ev.Overlaps(w) {
			out = append(out, ev)
		}
	}
	return out, nil
}

func (m *Mock) ListReminders(ctx context.Context, w Window) ([]Reminder, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	if err := w.Valid(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Reminder
	for _, r := range m.reminders {
		if !r.TriggerAt.Before(w.From) && r.TriggerAt.Before(w.To) {
			out = append(out, r)
		}
	}
	return out, nil
}
