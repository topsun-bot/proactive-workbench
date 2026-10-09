# Proactivity core

Sense → generate a goal → plan → decide whether to interrupt.

```
perception  →  goal  →  plan  →  interrupt policy
     ↑                               │
     └──────── re-tick (CLI --repeat) ┘
```

This package does **not** replace Shaoruru’s `internal/planner` (which routes a user-typed goal to `flow/umbrella`). This loop **proposes** a goal from sensors. Wiring a proposed `umbrella_reminder` into `umbrella.Execute` is an integration point, not done here, so the umbrella demo stays untouched.

## One tick

`Tick` reads:

- `situation.Source` — place / activity (fixtures: home + idle)
- `weather.Source` — current hour + tomorrow 08:00
- `calendar.Source` — events in the next 24 hours

then picks at most one goal, builds a short plan, and applies `Policy`.

## Goals (rule-based, no LLM)

| Kind | When |
|------|------|
| `visit_park` | At home, idle, outdoor-OK weather, weekend or after 16:00, no meeting in 2 hours |
| `umbrella_reminder` | Rain/storm tomorrow 08:00 and no existing umbrella calendar title |
| `none` | Nothing worth acting on |

Scores are additive. Default interrupt threshold is **70**.

## Interrupt policy

A tick **does not interrupt** when any of these hold:

1. Goal is `none`
2. Score &lt; `MinScore`
3. Quiet hours (`QuietStart`–`QuietEnd`, default 22:00–08:00, wrapping midnight)
4. **Situation unchanged** — fingerprint equals the last tick’s fingerprint (dedupe)

Fingerprint = `YYYY-MM-DD|place|weather|goal|sorted event UIDs` (no temperature, so a 0.1°C flap does not re-notify).

Quiet hours and dedupe are the “only interrupt when it matters” controls. The owner’s example (home, bored, good weather → park) interrupts on the first tick and is silent on the second if nothing changed.

## CLI

```
go run ./cmd/proactivity tick
go run ./cmd/proactivity tick --now=2026-10-10T15:00:00+08:00 --repeat=2
go run ./cmd/proactivity tick --weather=rain --now=2026-10-09T13:00:00+08:00
```

Always uses mock/fixture sources. No live network.
