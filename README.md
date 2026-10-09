# Proactive Workbench

跨平台主动性工作台。主动性 agent 统一管理日历、天气、闹钟等工具，感知上下文、自主生成目标并持续规划。核心卖点是**跨工具协作**：例如「明天早上八点提醒带伞」会同时用到天气、日历和闹钟。

## Tech choice

**Go CLI on Linux** (no desktop GUI, no macOS, no Swift).

A Mac is not available for this milestone, and a GUI would not run headlessly in CI. A single static Go binary:

- builds and runs on `ubuntu-latest` and on this Linux development environment
- needs no display, certificates, or runtime (CGO is off)
- is easy to demo: `workbench demo` prints the full umbrella plan
- packages as a `.tar.gz` artifact without AppImage/deb machinery

The architecture still matches the original workbench idea: a **tool plugin interface**, a **planner** that routes a goal to a flow, and stub tools that later become real integrations.

Minimum: Go 1.22, Linux. Windows/macOS clients are out of scope here.

## Layout

```
cmd/workbench/             CLI (demo, plan, tools, version)
internal/tool/             WorkbenchTool protocol + registry
internal/tools/weather|calendar|alarm
internal/flow/umbrella     Cross-tool demo
internal/planner           Goal → flow
internal/intent            Phrase matching (EN + 中文)
scripts/package-linux.sh   linux/amd64 tar.gz
.github/workflows/         ubuntu-latest CI
```

## Plugin interface

Every tool implements `tool.Tool`:

- `Descriptor()` — id, name, summary
- `Handle(Request) (Result, error)` — generic action + string payload

| Tool | Implementation | Notes |
|------|----------------|--------|
| Weather | `internal/tools/weather` | **MOCK — not live data.** Every forecast has `IsMock: true` and source label `MOCK weather (not live data)`. Scenarios: rain / clear / cloudy. |
| Calendar | `internal/tools/calendar` | In-memory only. |
| Alarm | `internal/tools/alarm` | In-memory only. |

## Cross-tool example

**「明天早上八点提醒带伞」** / **bring an umbrella tomorrow 8am**

1. Resolve *tomorrow 08:00* in the given timezone (default `Asia/Shanghai`).
2. Ask the **mock** weather tool for that morning.
3. If the mock forecast is **rain**: create an in-memory calendar event **and** an alarm.
4. If **clear** or **cloudy**: skip both reminders and print why.

```bash
go run ./cmd/workbench demo
go run ./cmd/workbench demo --weather=clear
go run ./cmd/workbench plan "bring an umbrella tomorrow 8am" --weather=rain
go run ./cmd/workbench tools
```

`--now=RFC3339` freezes the clock (CI uses `2026-10-09T13:00:00+08:00` so tomorrow is `2026-10-10 08:00`).

## Tests

```bash
go test ./...
```

Coverage: rain → calendar+alarm, clear/cloudy → skip, EN/中文 intent, mock label on the weather plugin, CLI demo output.

## Linux package

```bash
./scripts/package-linux.sh
# dist/proactive-workbench-0.1.0-linux-amd64.tar.gz
tar -xzf dist/proactive-workbench-0.1.0-linux-amd64.tar.gz
./proactive-workbench-0.1.0-linux-amd64/workbench demo
```

## CI

On every **push** and **workflow_dispatch**, `.github/workflows/linux-build.yml` runs on **ubuntu-latest** only:

1. `go test ./...`
2. Runs the umbrella demo twice (rain and clear) so the plan text is in the job log
3. Builds `proactive-workbench-0.1.0-linux-amd64.tar.gz` and uploads it as an Actions artifact
4. Extracts the archive and runs `workbench version` / `workbench tools` to prove the artifact is executable

There are no macOS runners, DMGs, or code signing steps.

## Status

First-batch tool scope, live weather/calendar backends, and the proactive “when to interrupt” policy are still open (`TODO.md` items 5–7).
