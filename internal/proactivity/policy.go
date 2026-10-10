package proactivity

import (
	"fmt"

	"github.com/topsun-bot/proactive-workbench/internal/memory"
)

// Policy is the interrupt rule set. Zero value is not valid; use DefaultPolicy.
type Policy struct {
	MinScore   int
	QuietStart int // inclusive hour 0–23
	QuietEnd   int // exclusive hour 0–23; may be < QuietStart (wraps midnight)
}

func DefaultPolicy() Policy {
	return Policy{MinScore: 70, QuietStart: 22, QuietEnd: 8}
}

func (p Policy) Validate() error {
	if p.MinScore < 0 || p.MinScore > 200 {
		return fmt.Errorf("proactivity: MinScore %d out of range", p.MinScore)
	}
	if p.QuietStart < 0 || p.QuietStart > 23 || p.QuietEnd < 0 || p.QuietEnd > 23 {
		return fmt.Errorf("proactivity: quiet hours must be hours 0–23")
	}
	return nil
}

// WithMemory overlays quiet hours from local memory preferences when set.
func (p Policy) WithMemory(snap memory.Snapshot) Policy {
	start, end, ok := snap.QuietHours()
	if !ok {
		return p
	}
	if start >= 0 {
		p.QuietStart = start
	}
	if end >= 0 {
		p.QuietEnd = end
	}
	return p
}

func (p Policy) InQuietHours(hour int) bool {
	if hour < 0 || hour > 23 {
		return false
	}
	if p.QuietStart == p.QuietEnd {
		return false
	}
	if p.QuietStart < p.QuietEnd {
		return hour >= p.QuietStart && hour < p.QuietEnd
	}
	// Wraps midnight, e.g. 22 → 8.
	return hour >= p.QuietStart || hour < p.QuietEnd
}
