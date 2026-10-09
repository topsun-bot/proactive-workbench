package proactivity

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Memory stores the last interrupt fingerprint so an unchanged situation
// does not re-notify. In-process only; the CLI reuses one Memory across --repeat.
type Memory struct {
	mu     sync.Mutex
	lastFP string
}

func NewMemory() *Memory { return &Memory{} }

func (m *Memory) LastFingerprint() string {
	if m == nil {
		return ""
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastFP
}

func (m *Memory) Remember(fp string) {
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

func Decide(p Perception, g Goal, policy Policy, mem *Memory) Decision {
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
