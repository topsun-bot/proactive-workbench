package proactivity

import (
	"sync"
	"time"

	"github.com/topsun-bot/proactive-workbench/internal/clock"
	"github.com/topsun-bot/proactive-workbench/internal/memory"
)

type RoutineKind string

const (
	RoutineMorningBrief RoutineKind = "morning_brief"
	RoutineSenseTick    RoutineKind = "sense_tick"
)

const DefaultSenseInterval = 15 * time.Minute

// Routine is a local schedule entry. morning_brief fires once per local day
// at Hour:Minute. sense_tick fires when Interval has elapsed since LastRun.
type Routine struct {
	Kind     RoutineKind   `json:"kind"`
	Hour     int           `json:"hour,omitempty"`
	Minute   int           `json:"minute,omitempty"`
	Interval time.Duration `json:"-"`
}

type RoutineStatus struct {
	Routine
	IntervalSeconds int    `json:"intervalSeconds,omitempty"`
	Due             bool   `json:"due"`
	LastRun         string `json:"lastRun,omitempty"`
}

// Scheduler is in-process. Last-run times are not written to disk.
type Scheduler struct {
	clk      clock.Clock
	mu       sync.Mutex
	last     map[RoutineKind]time.Time
	interval time.Duration
}

func NewScheduler(clk clock.Clock, senseEvery time.Duration) *Scheduler {
	if senseEvery <= 0 {
		senseEvery = DefaultSenseInterval
	}
	return &Scheduler{
		clk:      clk,
		last:     map[RoutineKind]time.Time{},
		interval: senseEvery,
	}
}

func (s *Scheduler) now() time.Time {
	if s == nil || s.clk.Now == nil {
		return time.Time{}
	}
	loc := s.clk.Location
	if loc == nil {
		loc = time.UTC
	}
	return s.clk.Now().In(loc)
}

func DefaultRoutines(snap memory.Snapshot) []Routine {
	h, m := snap.MorningBriefClock()
	return []Routine{
		{Kind: RoutineMorningBrief, Hour: h, Minute: m},
		{Kind: RoutineSenseTick, Interval: DefaultSenseInterval},
	}
}

func (s *Scheduler) Status(snap memory.Snapshot) []RoutineStatus {
	now := s.now()
	out := make([]RoutineStatus, 0, 2)
	for _, r := range DefaultRoutines(snap) {
		if r.Kind == RoutineSenseTick {
			r.Interval = s.interval
		}
		st := RoutineStatus{Routine: r, Due: s.due(r, now)}
		if r.Interval > 0 {
			st.IntervalSeconds = int(r.Interval.Seconds())
		}
		s.mu.Lock()
		if last, ok := s.last[r.Kind]; ok && !last.IsZero() {
			st.LastRun = last.Format(time.RFC3339)
		}
		s.mu.Unlock()
		out = append(out, st)
	}
	return out
}

func (s *Scheduler) Due(snap memory.Snapshot) []Routine {
	var due []Routine
	for _, st := range s.Status(snap) {
		if st.Due {
			due = append(due, st.Routine)
		}
	}
	return due
}

func (s *Scheduler) MarkRan(kind RoutineKind, at time.Time) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.last == nil {
		s.last = map[RoutineKind]time.Time{}
	}
	s.last[kind] = at
}

func (s *Scheduler) Last(kind RoutineKind) time.Time {
	if s == nil {
		return time.Time{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.last[kind]
}

func (s *Scheduler) due(r Routine, now time.Time) bool {
	if now.IsZero() {
		return false
	}
	s.mu.Lock()
	last := s.last[r.Kind]
	s.mu.Unlock()

	switch r.Kind {
	case RoutineMorningBrief:
		start := time.Date(now.Year(), now.Month(), now.Day(), r.Hour, r.Minute, 0, 0, now.Location())
		if now.Before(start) {
			return false
		}
		if last.IsZero() {
			return true
		}
		ly, lm, ld := last.In(now.Location()).Date()
		ny, nm, nd := now.Date()
		return ly != ny || lm != nm || ld != nd
	case RoutineSenseTick:
		interval := r.Interval
		if interval <= 0 {
			interval = s.interval
		}
		if last.IsZero() {
			return true
		}
		return !now.Before(last.Add(interval))
	default:
		return false
	}
}
