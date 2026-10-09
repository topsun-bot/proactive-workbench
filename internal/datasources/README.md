# `internal/datasources`

Read-only environment feeds for the Linux workbench.

- **Weather:** `weather.Source` — mock fixtures + Open-Meteo adapter (keyless). Tests never hit the network.
- **Calendar / reminders:** `calendar.Source` — mock + local ICS (recommended Linux path). CalDAV is a documented stub.
- **Situation:** `situation.Source` — place/activity fixtures (not GPS).

Design, API choice, and Linux calendar tradeoffs: [DESIGN.md](DESIGN.md).

This is a sibling of Shaoruru’s `internal/tools/*` plugins, not a replacement. Optional `tool.Tool` wrappers live next to each source so the existing registry can adopt them later.
