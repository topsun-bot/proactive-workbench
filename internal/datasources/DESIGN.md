# Data sources (Linux) — weather and calendar/reminders

Owner: A4 技术互联 (items 6–7). This package is **not** Shaoruru’s workbench plugins.

Shaoruru already shipped `internal/tools/weather` and `internal/tools/calendar` as **in-memory plugin stubs** behind `tool.Tool` (`Handle(action, payload)`). Those stay the planner-facing plugins for the umbrella demo.

This directory is the **data-source layer** those plugins (or the proactivity core) can sit on later:

| Layer | Package | Role |
|-------|---------|------|
| Plugin (Shaoruru) | `internal/tools/{weather,calendar,alarm}` | Workbench actions: forecast / createEvent / createAlarm |
| Data source (this) | `internal/datasources/{weather,calendar,situation}` | Read environment: forecast snapshot, events/reminders, place/activity |
| Core (this) | `internal/proactivity` | sense → goal → plan → interrupt decision |

Do not import this package from Shaoruru’s files yet. Optional adapters that implement `tool.Tool` live here (`weather/plugin.go`, `calendar/plugin.go`) so the desktop/CLI client can register a live or fixture-backed plugin without rewriting the registry.

---

## Weather API

### Candidates

| Provider | Auth | Linux / CI | Notes |
|----------|------|------------|--------|
| **Open-Meteo** | None (keyless) | HTTPS GET | Forecast + WMO codes. Fair-use free tier. No signup. |
| OpenWeatherMap | API key | HTTPS GET | Key in CI/secrets; ToS signup. Better UX maps, worse for this repo. |
| NOAA / api.weather.gov | None | HTTPS GET | US points only. Default workbench TZ is `Asia/Shanghai`. |
| wttr.in | None | HTTPS | Human/text-oriented; not a stable JSON contract. |
| Apple WeatherKit | Developer account | Not Linux-native | Needs Apple certs; Mac items are parked. |

### Choice: Open-Meteo

**Why:** no API key (CI and this Linux environment stay secret-free), works worldwide, documented JSON, WMO `weather_code`, hourly precipitation probability — enough to decide “good enough to go outside” vs “rain tomorrow 08:00, pack an umbrella”. Matches the project’s `CGO_ENABLED=0` static binary: one `net/http` GET, stdlib JSON.

**Endpoint (forecast):** `GET https://api.open-meteo.com/v1/forecast`

Required query (what the adapter sends):

- `latitude`, `longitude`
- `current=temperature_2m,weather_code,precipitation`
- `hourly=temperature_2m,weather_code,precipitation_probability,precipitation`
- `forecast_days=2`
- `timezone=<IANA>`

**Default coordinates** are Shanghai Changning District (`31.2205, 121.4248`)
from Wikipedia “Changning, Shanghai” (Changning NPC Committee,
31°13′14″N 121°25′29″E). They line up with `--tz=Asia/Shanghai`. They are
**not** a GPS fix. Override with config `lat`/`lon`. Label every snapshot
with the location label and `IsMock` / `Source`.

**Commands:** `workbench today` / `workbench serve` / `proactivity *`
call `NewLiveOpenMeteo` by default. A failed fetch surfaces
`weatherCondition=unavailable` and `天气暂时查不到` — never fixture numbers.
Fixtures only under `--debug-fixture` (or `--weather` / `PW_DEBUG_FIXTURE`).

**Tests:** parse fixture JSON and talk to an injected `Doer`. Unit tests never call `api.open-meteo.com`.

**WMO codes** (Open-Meteo / WMO-4677, adapter mapping):

| Codes | Condition |
|-------|-----------|
| 0, 1 | clear |
| 2, 3 | cloudy |
| 45, 48 | fog |
| 51–67, 80–82 | rain |
| 71–77, 85–86 | snow |
| 95–99 | storm |
| other | unknown |

Fixtures under `weather/fixtures/` are **synthetic schema-correct payloads**, not observations. See that folder’s README.

---

## Calendar / reminders on Linux

**EventKit is not in this Linux package.** Shaoruru implements EventKit in
`macos/` (PR #5) against `calendar.Source`. There is no EventKit type, no
fake EventKit stub, and no CGO bridge here. Linux stays on local ICS.

### Candidates

| Path | Needs | Works with `CGO_ENABLED=0`? | Auth / permissions | Verdict |
|------|-------|-----------------------------|--------------------|---------|
| **Local ICS file or directory** (Thunderbird export, [vdirsyncer](https://vdirsyncer.pimutils.org/) cache, Nextcloud Files, `khal`) | Read filesystem | Yes | File permissions only | **Recommended now** |
| **CalDAV** (`REPORT calendar-query`) | URL + username/password or app password | Yes (HTTP) | User credentials; store outside the repo | **Recommended next** — same events as ICS once synced |
| Evolution Data Server | `libecal` + session bus + CGO | **No** | Desktop session | Reject for this binary |
| KDE Akonadi | CGO + Akonadi | **No** | Desktop session | Reject for this binary |
| Google Calendar API | OAuth client + refresh token | Yes | Google cloud project; not Linux-specific | Optional later, not the Linux path |
| `notify-send` / systemd timers | session | Yes | user session | Delivery, not a calendar store |

### Recommendation

1. **Read path (this PR):** parse RFC 5545 **ICS** from a file or a directory of `.ics` files. This is what vdirsyncer writes, what Thunderbird/Evolution export, and what a Nextcloud calendar download is. No daemon, no CGO, no secrets in CI.
2. **Sync path (follow-up):** CalDAV against Nextcloud / Fastmail / Google CalDAV, **or** keep using vdirsyncer and only read the local directory. The `CalDAV` type in this package is a **stub** (never dials). It exists so the interface is stable when someone adds credentials.
3. **Do not** link Evolution Data Server or Akonadi. They break the static Linux artifact Shaoruru’s CI publishes.

Reminders: ICS `VALARM` (DISPLAY/AUDIO) attached to `VEVENT`. There is no separate Linux “Reminders.app”. Evolution/Thunderbird tasks (`VTODO`) can be added later on the same parser.

**Permissions / risk**

- ICS files may contain private titles and locations. Treat them as user data; do not log full event dumps in CI.
- CalDAV passwords must come from the environment or a secret store, never from the repo.
- Google OAuth would add a cloud project and a stored refresh token — extra moving parts for no Linux-native gain.

**Mocks / fixtures:** `calendar/fixtures/sample.ics` is labeled `FIXTURE` in `PRODID`, UIDs, and summaries. Not a real appointment.

---

## Situation (place / activity)

Location stays **fixture-only**, clearly labeled

`MOCK location — fixture-only, not a live GPS or GeoClue fix`.

Do **not** wire GeoClue, CoreLocation, or any live location API. Default
fixture is `home` + `idle`. The path is injectable for tests via
`situation.NewMock`.

---

## Integration points for Shaoruru

1. Register `weather.NewTool(src)` / `calendar.NewListTool(src)` on the existing `tool.Registry` when ready to swap stubs for fixture or Open-Meteo/ICS backends. The umbrella demo can keep using `internal/tools/weather.New(scenario)` until then.
2. `internal/clock` is reused so `--now` / `--tz` stay consistent with `workbench demo`.
3. A generated `umbrella_reminder` goal is the proactive form of 「明天早上八点提醒带伞」. Executing it should call the existing `internal/flow/umbrella` planner, not a second calendar writer.
4. Do not put network or file I/O inside `internal/tools/*` — keep plugins thin; inject a `datasources` implementation.
5. EventKit lives in `macos/` against `calendar.Source`. Do not add an EventKit stub here.
6. Location stays MOCK. Do not add GeoClue.
