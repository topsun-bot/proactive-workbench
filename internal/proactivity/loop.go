package proactivity

import (
	"context"
	"fmt"

	"github.com/topsun-bot/proactive-workbench/internal/datasources/weather"
	"github.com/topsun-bot/proactive-workbench/internal/memory"
)

// Tick runs one perceive → goal → plan → decide cycle against the injected sensors.
// Memory is empty: preferences default (likes outdoors, policy quiet hours).
func Tick(ctx context.Context, sensors Sensors, policy Policy, mem *Dedupe) (Result, error) {
	return TickWithMemory(ctx, sensors, policy, mem, memory.Empty())
}

// TickWithMemory is Tick plus local long-term memory (preferences, people, goals, commitments).
func TickWithMemory(ctx context.Context, sensors Sensors, policy Policy, mem *Dedupe, snap memory.Snapshot) (Result, error) {
	policy = policy.WithMemory(snap)
	if err := policy.Validate(); err != nil {
		return Result{}, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	p, err := Perceive(ctx, sensors)
	if err != nil {
		return Result{}, err
	}
	g := GenerateGoal(p, snap)
	plan := BuildPlan(p, g)
	d := Decide(p, g, policy, mem)
	if d.Interrupt {
		mem.Remember(d.Fingerprint)
	}
	return Result{Perception: p, Goal: g, Plan: plan, Decision: d}, nil
}

// FormatResult is the human CLI report.
func FormatResult(r Result, tickNo int) string {
	var b []byte
	w := func(format string, args ...any) {
		b = append(b, []byte(fmt.Sprintf(format, args...))...)
	}
	if tickNo > 0 {
		w("Tick %d\n", tickNo)
	} else {
		w("Proactivity tick\n")
	}
	w("================\n")
	w("Clock:     %s\n", r.Perception.At.Format("2006-01-02 15:04 MST"))
	w("Place:     %s  [%s]\n", r.Perception.Situation.Place, r.Perception.Situation.Source)
	w("Activity:  %s\n", r.Perception.Situation.Activity)
	if r.Perception.Weather.Available() {
		w("Weather:   %s %.1f°C  [%s]\n",
			r.Perception.NowWeather.Condition.DisplayName(),
			r.Perception.NowWeather.TemperatureC,
			r.Perception.Weather.Source,
		)
		w("Tomorrow8: %s %.1f°C\n",
			r.Perception.TomorrowAM.Condition.DisplayName(),
			r.Perception.TomorrowAM.TemperatureC,
		)
	} else {
		w("Weather:   %s  [%s]\n", weather.UserFacingUnavailable, r.Perception.Weather.Source)
		if r.Perception.Weather.Error != "" {
			w("           error: %s\n", r.Perception.Weather.Error)
		}
	}
	if r.Perception.CalendarError != "" {
		w("Calendar:  %s (%s)\n", r.Perception.CalendarUserMessage, r.Perception.CalendarError)
	} else {
		w("Calendar:  %d event(s) in next 24h\n", len(r.Perception.Events))
	}
	for _, ev := range r.Perception.Events {
		w("           - %s (%s)\n", ev.Title, ev.Start.Format("15:04"))
	}
	w("\nGoal:      %s  (score %d)\n", r.Goal.Kind, r.Goal.Score)
	w("           %s\n", r.Goal.Reason)
	w("\nPlan:\n")
	for i, step := range r.Plan.Steps {
		w("  %d. %s\n", i+1, step)
	}
	mark := "NO"
	if r.Decision.Interrupt {
		mark = "YES"
	}
	w("\nInterrupt: %s\n", mark)
	w("  Reason:  %s\n", r.Decision.Reason)
	w("  Fingerprint: %s\n", r.Decision.Fingerprint)
	return string(b)
}
