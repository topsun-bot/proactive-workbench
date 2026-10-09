# Proactivity external interface

The core serves two clients: the Linux CLI (`cmd/proactivity`) and a future
macOS client (Shaoruru). Platform I/O (EventKit, notification banners, live
GPS) is **not** in this package. Inject `weather.Source`, `calendar.Source`,
`situation.Source`, `clock.Clock`, and `memory.Store`.

## Why loopback HTTP/JSON

| Option | Verdict |
|--------|---------|
| Go package API only | Fine for the Linux CLI; Swift cannot import it |
| CLI `--json` only | Works, but a long-lived Mac app would spawn a process per call |
| JSON-over-stdio | Awkward for concurrent brief + tick + memory writes |
| **127.0.0.1 HTTP/JSON** | Language-neutral, `URLSession` on macOS, curl on Linux, OpenAPI-shaped |

**Chosen boundary:** localhost HTTP/JSON (`application/json`) bound to
`127.0.0.1` only (default port `8741`). The same payloads are also emitted by
`proactivity tick --json` and `proactivity brief --json`.

The stable Go API is `proactivity.Core` (`Tick`, `Brief`, `Memory`,
`UpdateMemory`, `Routines`, `RunDue`).

## Envelope

Every HTTP response and `--json` document:

```json
{"ok": true, "data": { }}
{"ok": false, "error": "human-readable message"}
```

`api` version is `1` (`GET /v1/health`). Additive fields are allowed; renaming
or removing a field is a breaking change.

## Endpoints

| Method | Path | Body | `data` |
|--------|------|------|--------|
| GET | `/v1/health` | — | `{service, api, bound}` |
| POST | `/v1/tick` | empty | `WireTick` |
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

## `WireTick`

```json
{
  "at": "2026-10-10T15:00:00+08:00",
  "place": "home",
  "activity": "idle",
  "weatherCondition": "clear",
  "weatherTempC": 22.0,
  "weatherSource": "FIXTURE weather — not live data",
  "calendarCount": 0,
  "events": [{"uid": "", "title": "", "start": ""}],
  "goalKind": "visit_park",
  "goalTitle": "Go to the park",
  "goalReason": "...",
  "goalScore": 110,
  "planSteps": ["..."],
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

## Routines

- `morning_brief` — once per local calendar day at the configured hour:minute
  (default 08:00, overridable via memory).
- `sense_tick` — every 15 minutes from last run (first run is due immediately).

Last-run is in-process only (injected `clock.Clock`). It is not persisted.

## Go API (same semantics)

```go
core, err := proactivity.NewCore(sensors, proactivity.DefaultPolicy(), store)
res, err := core.Tick(ctx)
brief, err := core.Brief(ctx)
snap, err := core.Memory()
```

Contract tests live in `contract_test.go`. No network: `httptest` + fixture
sensors + a temp JSON file.
