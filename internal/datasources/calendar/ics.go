package calendar

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ICS reads one .ics file or a directory of .ics files (vdirsyncer / Thunderbird export).
type ICS struct {
	Path      string
	Location  *time.Location
	Source    string
	IsFixture bool
}

func NewICS(path string) (*ICS, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("calendar: ICS path is required")
	}
	return &ICS{Path: path, Location: time.UTC, Source: "ics:" + path}, nil
}

func (s *ICS) ListEvents(ctx context.Context, w Window) ([]Event, error) {
	events, _, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	if err := w.Valid(); err != nil {
		return nil, err
	}
	var out []Event
	for _, ev := range events {
		if ev.Overlaps(w) {
			out = append(out, ev)
		}
	}
	return out, nil
}

func (s *ICS) ListReminders(ctx context.Context, w Window) ([]Reminder, error) {
	_, reminders, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	if err := w.Valid(); err != nil {
		return nil, err
	}
	var out []Reminder
	for _, r := range reminders {
		if !r.TriggerAt.Before(w.From) && r.TriggerAt.Before(w.To) {
			out = append(out, r)
		}
	}
	return out, nil
}

func (s *ICS) load(ctx context.Context) ([]Event, []Reminder, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
	}
	if s == nil || s.Path == "" {
		return nil, nil, fmt.Errorf("calendar: ICS path is required")
	}
	info, err := os.Stat(s.Path)
	if err != nil {
		return nil, nil, fmt.Errorf("calendar: stat ICS path: %w", err)
	}
	var files []string
	if info.IsDir() {
		entries, err := os.ReadDir(s.Path)
		if err != nil {
			return nil, nil, err
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if strings.EqualFold(filepath.Ext(name), ".ics") {
				files = append(files, filepath.Join(s.Path, name))
			}
		}
	} else {
		files = []string{s.Path}
	}

	loc := s.Location
	if loc == nil {
		loc = time.UTC
	}
	source := s.Source
	if source == "" {
		source = "ics:" + s.Path
	}
	var events []Event
	var reminders []Reminder
	for _, f := range files {
		body, err := os.ReadFile(f)
		if err != nil {
			return nil, nil, fmt.Errorf("calendar: read %s: %w", f, err)
		}
		evs, rems, err := ParseICS(body, source, s.IsFixture, loc)
		if err != nil {
			return nil, nil, fmt.Errorf("calendar: parse %s: %w", f, err)
		}
		events = append(events, evs...)
		reminders = append(reminders, rems...)
	}
	return events, reminders, nil
}
