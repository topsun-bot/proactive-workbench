# Proactivity external interface

The core serves two clients: the Linux CLI (`cmd/proactivity`) and Shaoruru’s
macOS client (`macos/` in PR #5). Platform I/O (EventKit, notification
banners, live GPS / GeoClue) is **not** in this package. Inject
`weather.Source`, `calendar.Source`, `situation.Source`, `clock.Clock`,
and `memory.Store`.

**EventKit:** nothing in this Linux core. Shaoruru implements EventKit
against `calendar.Source` in `macos/`. There is no fake EventKit stub here.

**Location:** fixture-only, labeled `MOCK location — fixture-only, not a live GPS or GeoClue fix`. No GeoClue.

## Why loopback HTTP/JSON

| Option | Verdict |
|--------|---------|
| Go package API only | Fine for the Linux CLI; Swift cannot import it |
| CLI `--json` only | Works, but a long-lived Mac app would spawn a process per call |
| JSON-over-stdio | Awkward for concurrent brief + tick + memory writes |
| **127.0.0.1 HTTP/JSON** | Language-neutral, `URLSession` on macOS, curl on Linux |

**Chosen boundary:** localhost HTTP/JSON (`application/json`) bound to
`127.0.0.1` only (default port `8741`). The Mac UI talks HTTP only — do
not call `proactivity.Core` from ObjC/Swift. Keep `Core` as the in-process
implementation *behind* `proactivity serve`.

The same payloads are also emitted by `proactivity tick --json` and
`proactivity brief --json`.

The stable Go API is `proactivity.Core` (`Tick`, `Today`, `Brief`,
`Memory`, `UpdateMemory`, `Routines`, `RunDue`).

## Envelope

Every `/v1/*` HTTP response and `--json` document:

```json
{"ok": true, "data": { }}
{"ok": false, "error": "human-readable message"}
```

`GET /api/today` is the exception: it returns the Today object **bare**
(no `{ok,data}` wrapper) so Shaoruru’s `NotificationGate` can read
`suggestions[]` at the document root. `GET /v1/today` wraps the same
object in the envelope.

`api` version is `1` (`GET /v1/health`). Additive fields are allowed;
renaming or removing a field is a breaking change.

## Route mapping (Shaoruru `/api/today`)

| Client path | This server | Notes |
|-------------|-------------|--------|
| `GET /api/today` | **`GET /api/today`** (alias, added) | Bare `WireToday`. `suggestions[]` each have `propose` (bool) and `reason` (string). |
| — | `GET /v1/today` | Same `WireToday` inside the envelope. |
| — | `POST /v1/tick` | One sense cycle. Also includes `propose` and `reason` on the tick object. |

`POST /v1/tick` is **not** an alias of `/api/today` (different method and
shape). The Mac client should keep calling `GET /api/today`.

## Endpoints

| Method | Path | Body | `data` / body |
|--------|------|------|----------------|
| GET | `/v1/health` | — | `{service, api, bound}` |
| POST | `/v1/tick` | empty | `WireTick` (includes `propose`, `reason`) |
| GET | `/v1/today` | — | `WireToday` (envelope) |
| GET | `/api/today` | — | `WireToday` **bare** (no envelope) |
| GET | `/v1/brief` | — | `Brief` |
| GET | `/v1/memory` | — | `memory.Snapshot` |
| POST | `/v1/memory/commitments` | `Commitment` | snapshot |
| DELETE | `/v1/memory/commitments/{id}` | — | snapshot |
| POST | `/v1/memory/people` | `Person` | snapshot |
| DELETE | `/v1/memory/people/{id}` | — | snapshot |
| POST | `/v1/memory/preferences` | `Preference` | snapshot |
| DELETE | `/v1/memory/preferences/{key}` | — | snapshot |
| POST | `/v1/memory/goals` | `LongTermGoal` | snapshot |
| DELETE | `/v1/memory/goals/{id}` | — | snapshot |
| GET | `/v1/routines` | — | `[]RoutineStatus` |
| POST | `/v1/routines/run` | empty | `[]WireRoutineRun` |

Non-loopback peers receive `403` `proactivity API is loopback-only`.
Listen addresses other than `127.0.0.1` / `localhost` / `::1` are rejected.

## First gate: `propose` and `reason`

These two fields are named **exactly** `propose` (bool) and `reason`
(string). They live on:

1. each object in `suggestions[]` from `GET /api/today` and `GET /v1/today`
2. the `WireTick` object from `POST /v1/tick` (and `proactivity tick --json`)

`propose` is the core’s first notify gate (quiet hours, min score, dedupe,
no actionable goal). It is **not** a final “show a banner” decision.

The Mac client owns the second gate: OS Focus / Do Not Disturb. A
user-visible banner should pass **both**. This package does not implement
Focus-mode.

`interrupt` on `WireTick` is a legacy alias of `propose` for existing CLI
tests. Prefer `propose`.

## `WireToday` / `suggestions[]`

```json
{
  "at": "2026-10-10T15:00:00+08:00",
  "place": "home",
  "activity": "idle",
  "locationSource": "MOCK location — fixture-only, not a live GPS or GeoClue fix",
  "weatherCondition": "clear",
  "weatherSource": "FIXTURE weather — not a live observation",
  "goalKind": "visit_park",
  "brief": { },
  "suggestions": [
    {
      "title": "Go to the park",
      "body": "At home, idle, weather is good, calendar is free.",
      "kind": "visit_park",
      "source": "proactivity.Core first gate (quiet hours / score / dedupe)",
      "propose": true,
      "reason": "new high-value suggestion (visit_park, score 110)"
    }
  ],
  "sourcesAreFixtures": true
}
```

There is always one suggestion object so `propose` and `reason` are
present even when the goal is `none` (`propose: false`).

## `WireTick`

```json
{
  "at": "2026-10-10T15:00:00+08:00",
  "place": "home",
  "activity": "idle",
  "weatherCondition": "clear",
  "weatherTempC": 22.0,
  "weatherSource": "FIXTURE weather — not a live observation",
  "calendarCount": 0,
  "events": [{"uid": "", "title": "", "start": ""}],
  "goalKind": "visit_park",
  "goalTitle": "Go to the park",
  "goalReason": "...",
  "goalScore": 110,
  "planSteps": ["..."],
  "propose": true,
  "reason": "new high-value suggestion (visit_park, score 110)",
  "interrupt": true,
  "decisionReason": "new high-value suggestion (visit_park, score 110)",
  "fingerprint": "2026-10-10|home|clear|visit_park|",
  "sourcesAreFixtures": true
}
```

## `Brief`

```json
{
  "date": "2026-10-10",
  "isFixture": true,
  "source": "FIXTURE morning brief — weather, calendar, and memory are fixtures",
  "weather": "Clear 22.0°C now; tomorrow 08:00 Clear 22.0°C [...]",
  "events": ["No calendar events in the next 24 hours."],
  "memory": ["FIXTURE memory — not a live user store"],
  "suggestion": "visit_park (score 115): ...",
  "sourcesAreFixtures": true
}
```

## `memory.Snapshot` (schema version 1)

```json
{
  "version": 1,
  "updatedAt": "2026-10-09T00:00:00Z",
  "isFixture": true,
  "source": "FIXTURE memory — not a live user store",
  "commitments": [{"id": "", "title": "", "when": "", "notes": "", "source": ""}],
  "people": [{"id": "", "name": "", "relation": "", "notes": "", "source": ""}],
  "preferences": [{"key": "likes_outdoors", "value": "true", "source": ""}],
  "goals": [{"id": "", "title": "", "status": "active", "notes": "", "source": ""}]
}
```

Well-known preference keys: `quiet_hours_start`, `quiet_hours_end`,
`likes_outdoors`, `morning_brief_hour`, `morning_brief_minute`.

## Routines and last-run persistence

- `morning_brief` — once per local calendar day at the configured hour:minute
  (default 08:00, overridable via memory).
- `sense_tick` — every 15 minutes from last run (first run is due immediately).

Last-run is persisted as JSON (schema version 1, atomic write). Path is
injectable (`NewCoreWithLastRun`, `--last-run-file`). Defaults:

| OS | Path |
|----|------|
| macOS | `~/Library/Application Support/Today Workbench/routines-last-run.json` |
| Linux | `$XDG_CONFIG_HOME/today-workbench/routines-last-run.json` (fallback `~/.config/…`) |

A missing or corrupt file is treated as empty (every routine is due).
`NewCore` (no path) stays in-process so tests do not write the real home
directory.

## `umbrella_reminder` execution

This package **proposes** `umbrella_reminder`. Hosts that execute it should
call existing `umbrella.Execute` (`internal/flow/umbrella`) through the
`tool.Tool` registry. Do not add a second calendar writer in this core.

## Go API (same semantics)

```go
core, err := proactivity.NewCore(sensors, proactivity.DefaultPolicy(), store)
// persist last-run:
core, err = proactivity.NewCoreWithLastRun(sensors, policy, store, lastRunPath)
res, err := core.Tick(ctx)
today, err := core.Today(ctx)
brief, err := core.Brief(ctx)
snap, err := core.Memory()
```

Contract tests live in `contract_test.go`. No network: `httptest` + fixture
sensors + a temp JSON file.
