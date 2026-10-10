# Proactive Workbench

主动性工作台：一个**纯软件**的工作台，不是机器人，没有四肢，就是一个持续运行的「大脑」。一套共用的 Go 主动 agent 内核，两个前端：Linux 命令行 + Mac 客户端（界面参考 [Today](https://today.ai/)）。

## 产品定义（2026-10-09 张益新确认；替代当天上午的「只做 Linux」）

1. **纯软件**：不控制任何硬件、机械或身体动作。
2. **核心是主动 agent**：持续感知环境，自主生成目标，持续规划，**不等人下指令**；先开口给建议，但先判断该不该打扰。
3. **能力向 Today 看齐**（首版已在 main，数据仍以标注 MOCK/fixture 的样例为主）：
   - **活的记忆**：日程、人、偏好、长期目标
   - **晨间简报**：每天早上主动给出当天概览
   - **先开口的建议** + 打扰判断（何时说、何时不说）
   - **定时例程**（routines）
4. **共用内核，两种交付物**：
   - Linux：Go 命令行，GitHub Actions `ubuntu-latest` 构建，产物 `linux-amd64` `.tar.gz`
   - Mac：Mac 客户端，GitHub Actions `macos-latest` 构建，产物为**未签名** DMG
5. **跨工具协作**：例如「明天早上八点提醒带伞」同时用到天气、日历和闹钟。按已批准的 `docs/PRD.md`，**不下雨也照样提醒**，文案说明降水概率。

## 范围

**做：**
- 共用 Go 主动性内核：感知 → 生成目标 → 规划 → 打扰判断；活的记忆、晨间简报、主动建议、定时例程
- 工具层：统一插件接口，首版工具见 `docs/PRD.md` / `config/scope.yaml`，之后逐步扩展
- Linux 命令行及 `ubuntu-latest` CI 构建、测试、打包（tar.gz）
- Mac 客户端，界面参考 Today：晨间简报、今日任务卡片、信号面板、例程列表、主动建议；与 Linux 共用同一内核
- `macos-latest` CI 构建**未签名** DMG

**后续（暂不做）：**
- Apple 开发者证书、签名、公证（见 `TODO.md` 第 8 项）

**不做（不在需求内）：**
- 机器人或任何实体动作：「跟随人」「取外卖」等全部取消
- 硬件、传感器、电机、四肢控制

## 架构（主动的「大脑」，均已在 main）

```
感知（时间、地点/活动、天气、日程） → 生成目标 → 规划 → 打扰策略（何时提醒人） → 通过工具层执行
```

- **工具层**（[PR #1](https://github.com/topsun-bot/proactive-workbench/pull/1)、[PR #2](https://github.com/topsun-bot/proactive-workbench/pull/2)）：`internal/tool` 插件接口 + weather（MOCK）/ calendar / alarm（内存）工具；`internal/planner` 通过 `tool.Registry` 把目标路由到 flow；`internal/flow/umbrella` 是跨工具演示。
- **数据源 + 主动性内核**（[PR #3](https://github.com/topsun-bot/proactive-workbench/pull/3)，A4 技术互联）：`internal/datasources`（weather / calendar / situation）、`internal/memory`（本地 JSON 记忆）、`internal/proactivity`（感知 → 目标 → 规划 → 打扰判断循环、晨间简报、例程调度、例程上次运行时间落盘、第一道打扰闸门 `propose` / `reason`、本机 HTTP 接口 `127.0.0.1:8741`，见 `internal/proactivity/API.md`），命令行入口 `cmd/proactivity`。
- **Mac 客户端 + Today 风格界面**（[PR #5](https://github.com/topsun-bot/proactive-workbench/pull/5)，Shaoruru）：`internal/today`（Snapshot、内嵌 web UI、`workbench today` / `workbench serve`），`macos/TodayWorkbench`（AppKit + WKWebView 壳、EventKit 日历、第二道通知闸门：只有 `propose=true` 且专注模式关闭才弹横幅），`scripts/package-macos.sh` 打未签名 DMG。

### 数据真实性（如实标注）

| 信号 | 现在 main 上的实际情况 |
|------|------------------------|
| 时间 | 真实时钟（`clock.Live`）；`--now` / `--debug-fixture` 只用于调试和 CI |
| 天气 | `internal/datasources/weather/openmeteo.go` 有 Open-Meteo 客户端（`NewLiveOpenMeteo`）和测试，但**目前没有任何命令接上它**。`workbench` 与 `proactivity` 用的都是录好的 Open-Meteo JSON fixture（clear / rain / cloudy），或 `internal/tools/weather` 的 MOCK；不传 `--weather` 时界面显示「unavailable」，内核按 cloudy fixture 处理，不编造晴天 |
| 日历 | Linux：`internal/datasources/calendar/ics.go` 有本地 ICS 读取（`NewICS`），命令行目前只接了内嵌的样例 `sample.ics`（`--calendar-fixture`），CalDAV 占位。Mac：客户端用 EventKit，拒绝授权时返回空列表并标 `fallback=calendar_permission_denied` |
| 地点 / 活动 | **MOCK**：`internal/datasources/situation` 只有 fixture，不开 GeoClue / CoreLocation，张益新同意前不启用定位 |
| 长期记忆 | 本地 JSON 文件（`--memory-file`）；演示数据是标注 MOCK 的 fixture |
| 睡眠、会议、身边的人 | 界面上的这些信号目前是 MOCK 样例 |

## Layout

```
cmd/workbench/             CLI: demo, plan, today, serve, tools, version
cmd/proactivity/           Proactivity core CLI: tick, brief, memory, serve
internal/tool/             WorkbenchTool protocol + registry
internal/tools/            weather (MOCK) | calendar | alarm (in-memory)
internal/flow/umbrella     Cross-tool umbrella flow
internal/planner           Goal → flow via tool.Registry
internal/intent            Phrase matching (EN + 中文)
internal/datasources/      weather (Open-Meteo client + fixtures) | calendar (ICS, CalDAV stub) | situation (MOCK)
internal/connector/        Unified 7-domain Connector contract, Registry, incremental SyncEngine, local test adapters
internal/memory/           Local JSON long-term memory
internal/proactivity/      sense → goal → plan → interrupt, brief, routines, HTTP API
internal/today/            Today-style snapshot, embedded web UI, port file
macos/TodayWorkbench/      AppKit/WKWebView shell, EventKit, notification gate
scripts/package-linux.sh   linux/amd64 tar.gz
scripts/package-macos.sh   unsigned DMG
tests/smoke/               Smoke & multi-scenario inspection scripts (build, serve liveness, rain/clear/unavailable/today)
.github/workflows/         CI: ubuntu-latest + macos-latest
```

## Plugin interface

The planner and umbrella flow call tools only through `tool.Registry` / `Handle`. A replacement registered under the same id (for example a live weather plugin) is used without rewriting the flow.

Every tool implements `tool.Tool`:

- `Descriptor()` — id, name, summary
- `Handle(Request) (Result, error)` — generic action + string payload

## Cross-tool example

**「明天早上八点提醒带伞」** / **bring an umbrella tomorrow 8am**（按 `docs/PRD.md`）

1. Resolve *tomorrow 08:00* in the given timezone (default `Asia/Shanghai`).
2. Ask the weather tool for that morning (MOCK scenario in the demo).
3. Rain: create the calendar event and alarm; the text mentions the precipitation chance.
4. Clear / cloudy: **still** create the reminders (`Outcome: no_rain`, e.g. 「今天降水概率 5%，可能用不上伞，带不带你定。」).
5. Weather unavailable: still remind, saying the weather couldn't be fetched.

```bash
go run ./cmd/workbench demo --weather=rain
go run ./cmd/workbench demo --weather=clear
go run ./cmd/workbench today --weather=clear
go run ./cmd/workbench serve            # 127.0.0.1:8741, real clock
go run ./cmd/proactivity brief --memory-fixture
go run ./cmd/workbench tools
```

`workbench serve` defaults to `127.0.0.1:8741`; if the port is taken it picks another and writes the actual `host:port` to `~/Library/Application Support/Today Workbench/port` (macOS) or `${XDG_CONFIG_HOME:-$HOME/.config}/today-workbench/port` (Linux), overridable with `PW_PORT_FILE`.

## Tests

```bash
go test ./...
./macos/test.sh   # macOS only
```

## Packages

```bash
./scripts/package-linux.sh   # dist/proactive-workbench-0.1.0-linux-amd64.tar.gz
./scripts/package-macos.sh   # unsigned DMG (macOS)
```

## CI

`.github/workflows/linux-build.yml` (workflow name `CI`) runs on every push and `workflow_dispatch`:

- **ubuntu-latest** — `go test ./...`, Mac client gate + EventKit-denied tests, umbrella demo (rain / clear / unavailable), `workbench today`, `proactivity tick` / `brief`, package and verify the tar.gz, upload artifact.
- **macos-latest** — unit tests, Mac client gate tests, package and verify the **unsigned** DMG, upload artifact. No signing or notarization.

## Status

- On main since 2026-10-10: PR #1, #2, #3, #5. TODO rows 2–7 and 15–17 are done.
- Still needs a person at the Mac (TODO 9): native window, calendar permission allow / deny, banner with Focus on / off.
- Not wired yet: live Open-Meteo, a real ICS / CalDAV calendar on Linux, real location. Smoke tests in `tests/smoke/` and workday inspection workflow are wired and verified (TODO 12 done).
