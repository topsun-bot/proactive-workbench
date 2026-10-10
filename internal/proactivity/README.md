# Proactivity core

Sense → generate a goal → plan → decide whether to interrupt.

```
perception + memory  →  goal  →  plan  →  interrupt policy + dedupe
        ↑                                          │
        └──── scheduler (morning_brief / sense_tick)
```

This package does **not** replace Shaoruru’s `internal/planner`. It proposes
a goal from sensors and local memory. External clients use `Core` or the
loopback JSON API (`API.md`).

## One tick

`Tick` / `TickWithMemory` reads situation, weather, calendar, then local
memory (preferences, people, commitments, long-term goals).

## Goals (rule-based, no LLM)

| Kind | When |
|------|------|
| `visit_park` | At home, idle, outdoor-OK weather, weekend or after 16:00, no meeting in 2 hours, `likes_outdoors` not false |
| `umbrella_reminder` | Rain/storm tomorrow 08:00 and no existing umbrella calendar title |
| `commitment_nudge` | A memory commitment starts within 2 hours (people names in the title raise score) |
| `none` | Nothing worth acting on |

Default interrupt threshold is **70**. Quiet hours come from `Policy` and
are overlaid from memory keys `quiet_hours_start` / `quiet_hours_end`.

## Interrupt policy (first gate)

A tick sets `propose=false` (and legacy `interrupt=false`) when any of these hold:

1. Goal is `none`
2. Score &lt; `MinScore`
3. Quiet hours (policy or memory)
4. Fingerprint equals the last interrupt (dedupe)

`propose` + `reason` are on each `suggestions[]` item from `GET /api/today`
and on `POST /v1/tick`. The Mac client applies OS Focus as a second gate.
This package does not implement Focus-mode.

Fingerprint = `YYYY-MM-DD|place|weather|goal|sorted event UIDs`.

## Morning brief

`BuildBrief` / `Core.Brief` writes a labeled daily briefing from weather +
next-24h calendar + memory. Fixture runs say
`FIXTURE morning brief — weather, calendar, and memory are fixtures`.

## Scheduler

`morning_brief` once per local day at the configured clock (default 08:00).
`sense_tick` every 15 minutes. Injected `clock.Clock`. Last-run persists
under `~/Library/Application Support/Today Workbench/` (macOS) or
`$XDG_CONFIG_HOME/today-workbench/` (Linux). Path is injectable for tests.

## CLI

```
go run ./cmd/proactivity tick --now=2026-10-10T15:00:00+08:00 --repeat=2
go run ./cmd/proactivity brief --memory-fixture --now=2026-10-10T08:00:00+08:00
go run ./cmd/proactivity tick --json
go run ./cmd/proactivity serve --listen=127.0.0.1:8741 --memory-fixture
```

Always fixture sensors unless a later adapter is injected. No live network
in tests.
