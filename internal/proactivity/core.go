package proactivity

import (
	"context"
	"fmt"
	"time"

	"github.com/topsun-bot/proactive-workbench/internal/clock"
	"github.com/topsun-bot/proactive-workbench/internal/memory"
)

// Core is the stable Go package API for the Linux CLI and a future macOS client.
// Platform I/O (EventKit, live GPS, notification banners) stays behind Sensors
// and memory.Store — this type does not import those.
type Core struct {
	Sensors   Sensors
	Policy    Policy
	Dedupe    *Dedupe
	Store     memory.Store
	Scheduler *Scheduler
}

func NewCore(sensors Sensors, policy Policy, store memory.Store) (*Core, error) {
	return NewCoreWithLastRun(sensors, policy, store, "")
}

// NewCoreWithLastRun is NewCore plus an injectable last-run file.
// An empty path keeps last-run in-process (tests). Production serve uses
// DefaultLastRunPath (macOS Application Support / Linux XDG config).
func NewCoreWithLastRun(sensors Sensors, policy Policy, store memory.Store, lastRunPath string) (*Core, error) {
	if err := sensors.valid(); err != nil {
		return nil, err
	}
	if store == nil {
		return nil, fmt.Errorf("proactivity: memory store is required")
	}
	if policy == (Policy{}) {
		policy = DefaultPolicy()
	}
	clk := sensors.Clock
	if clk.Now == nil {
		clk = clock.Live(time.UTC)
	}
	return &Core{
		Sensors:   sensors,
		Policy:    policy,
		Dedupe:    NewDedupe(),
		Store:     store,
		Scheduler: NewPersistentScheduler(clk, DefaultSenseInterval, lastRunPath),
	}, nil
}

// Today is one sense cycle plus the Today view Shaoruru’s client loads
// from GET /api/today. Each suggestions[] item carries propose + reason.
func (c *Core) Today(ctx context.Context) (WireToday, error) {
	res, err := c.Tick(ctx)
	if err != nil {
		return WireToday{}, err
	}
	brief, err := c.Brief(ctx)
	if err != nil {
		return WireToday{}, err
	}
	return ResultToToday(res, brief), nil
}

func (c *Core) snapshot() (memory.Snapshot, error) {
	if c == nil || c.Store == nil {
		return memory.Empty(), fmt.Errorf("proactivity: memory store is required")
	}
	return c.Store.Load()
}

func (c *Core) Tick(ctx context.Context) (Result, error) {
	snap, err := c.snapshot()
	if err != nil {
		return Result{}, err
	}
	return TickWithMemory(ctx, c.Sensors, c.Policy, c.Dedupe, snap)
}

func (c *Core) Brief(ctx context.Context) (Brief, error) {
	snap, err := c.snapshot()
	if err != nil {
		return Brief{}, err
	}
	p, err := Perceive(ctx, c.Sensors)
	if err != nil {
		return Brief{}, err
	}
	g := GenerateGoal(p, snap)
	return BuildBrief(p, snap, g), nil
}

func (c *Core) Memory() (memory.Snapshot, error) {
	return c.snapshot()
}

func (c *Core) UpdateMemory(fn func(*memory.Snapshot) error) (memory.Snapshot, error) {
	switch s := c.Store.(type) {
	case *memory.FileStore:
		return s.Update(fn)
	case *memory.MemStore:
		return s.Update(fn)
	default:
		snap, err := c.Store.Load()
		if err != nil {
			return memory.Snapshot{}, err
		}
		if err := fn(&snap); err != nil {
			return memory.Snapshot{}, err
		}
		if err := c.Store.Save(snap); err != nil {
			return memory.Snapshot{}, err
		}
		return snap, nil
	}
}

func (c *Core) Routines() ([]RoutineStatus, error) {
	snap, err := c.snapshot()
	if err != nil {
		return nil, err
	}
	return c.Scheduler.Status(snap), nil
}

// RunDue executes due routines (brief and/or a sense tick) and marks them ran.
func (c *Core) RunDue(ctx context.Context) ([]RoutineRun, error) {
	snap, err := c.snapshot()
	if err != nil {
		return nil, err
	}
	due := c.Scheduler.Due(snap)
	out := make([]RoutineRun, 0, len(due))
	now := c.Sensors.Clock.Now().In(c.Sensors.Clock.Location)
	for _, r := range due {
		run := RoutineRun{Kind: r.Kind, At: now.Format(time.RFC3339)}
		switch r.Kind {
		case RoutineMorningBrief:
			b, err := c.Brief(ctx)
			if err != nil {
				return out, err
			}
			run.Brief = &b
		case RoutineSenseTick:
			res, err := c.Tick(ctx)
			if err != nil {
				return out, err
			}
			run.Tick = &res
		default:
			return out, fmt.Errorf("proactivity: unknown routine %q", r.Kind)
		}
		c.Scheduler.MarkRan(r.Kind, now)
		out = append(out, run)
	}
	return out, nil
}

type RoutineRun struct {
	Kind  RoutineKind `json:"kind"`
	At    string      `json:"at"`
	Brief *Brief      `json:"brief,omitempty"`
	Tick  *Result     `json:"-"`
}
