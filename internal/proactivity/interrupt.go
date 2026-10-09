package proactivity

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Dedupe stores the last interrupt fingerprint so an unchanged situation
// does not re-notify. In-process only; the CLI reuses one Dedupe across --repeat.
// Long-term on-disk memory is internal/memory, not this type.
type Dedupe struct {
	mu     sync.Mutex
	lastFP string
}

func NewDedupe() *Dedupe { return &Dedupe{} }

// NewMemory is a deprecated alias kept so older call sites compile during the rename.
func NewMemory() *Dedupe { return NewDedupe() }

func (m *Dedupe) LastFingerprint() string {
	if m == nil {
		return ""
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastFP
}

func (m *Dedupe) Remember(fp string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastFP = fp
}

func Fingerprint(p Perception, g Goal) string {
	uids := make([]string, 0, len(p.Events))
	for _, ev := range p.Events {
		if ev.UID != "" {
			uids = append(uids, ev.UID)
		}
	}
	sort.Strings(uids)
	day := p.At.Format("2006-01-02")
	return strings.Join([]string{
		day,
		string(p.Situation.Place),
		string(p.NowWeather.Condition),
		string(g.Kind),
		strings.Join(uids, ","),
	}, "|")
}

func Decide(p Perception, g Goal, policy Policy, mem *Dedupe) Decision {
	fp := Fingerprint(p, g)
	d := Decision{Fingerprint: fp, Score: g.Score}

	if g.IsNone() {
		d.Reason = "no actionable goal"
		return d
	}
	if g.Score < policy.MinScore {
		d.Reason = fmt.Sprintf("score %d is below threshold %d", g.Score, policy.MinScore)
		return d
	}
	if policy.InQuietHours(p.At.Hour()) {
		d.Reason = fmt.Sprintf("quiet hours %02d:00–%02d:00", policy.QuietStart, policy.QuietEnd)
		return d
	}
	if mem != nil && mem.LastFingerprint() == fp {
		d.Reason = "situation unchanged (dedupe)"
		return d
	}

	d.Interrupt = true
	d.Reason = fmt.Sprintf("new high-value suggestion (%s, score %d)", g.Kind, g.Score)
	return d
}
