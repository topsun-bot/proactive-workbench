# Proactive Workbench

主动性工作台：一个**纯软件**的工作台，不是机器人，没有四肢，就是一个持续运行的「大脑」。一套共用的 Go 主动 agent 内核，两个前端：Linux 命令行 + Mac 客户端（界面参考 [Today](https://today.ai/)）。

## 产品定义（2026-10-09 张益新确认；替代当天上午的「只做 Linux」）

1. **纯软件**：不控制任何硬件、机械或身体动作。
2. **核心是主动 agent**：持续感知环境，自主生成目标，持续规划，**不等人下指令**；先开口给建议，但先判断该不该打扰。
3. **能力向 Today 看齐**（目标，逐步实现）：
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

## 目标架构（主动的「大脑」）

```
感知（时间、地点/活动、天气、日程） → 生成目标 → 规划 → 打扰策略（何时提醒人） → 通过工具层执行
```

- **工具层**（已在 main）：`internal/tool` 插件接口 + weather（mock）/ calendar / alarm（内存）桩，`internal/planner` 把目标路由到 flow，`internal/flow/umbrella` 是跨工具演示。
- **数据源 + 主动性内核**（未合并，见 [PR #3](https://github.com/topsun-bot/proactive-workbench/pull/3)，A4 技术互联，草稿）：提议 Open-Meteo 天气、本地 ICS 日历（CalDAV 先占位）、`internal/proactivity` 感知→目标→规划→打扰循环。合并前 main 上**没有**这些功能。
- **Mac 客户端**（尚未开始，见 `TODO.md` 第 15、16 项）：界面参考 Today，调用同一 Go 内核。main 上目前**没有** Mac 代码或 macOS CI。
- **Today 类能力**（记忆、晨间简报、主动建议、例程）：尚未实现，见 `TODO.md` 第 17 项。
- **评审修复**（未合并，见 [PR #2](https://github.com/topsun-bot/proactive-workbench/pull/2)，Shaoruru）。

## Tech choice

**Current main: Go CLI on Linux.** A Mac client sharing the same Go kernel is now in scope (2026-10-09) but not yet implemented.

For the first milestone a GUI would not run headlessly in CI. A single static Go binary:

- builds and runs on `ubuntu-latest` and on this Linux development environment
- needs no display, certificates, or runtime (CGO is off)
- is easy to demo: `workbench demo` prints the full umbrella plan
- packages as a `.tar.gz` artifact without AppImage/deb machinery

The architecture still matches the original workbench idea: a **tool plugin interface**, a **planner** that routes a goal to a flow, and stub tools that later become real integrations.

Minimum: Go 1.22, Linux. The Mac client is planned (TODO 15–16); Windows is out of scope.

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

The planner and umbrella flow call tools only through `tool.Registry` / `Handle`. A replacement registered under the same id (for example a live weather plugin) is used without rewriting the flow.

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

Coverage: rain → calendar+alarm, clear/cloudy → skip, EN/中文 intent, rejection of other umbrella times, planner via the plugin registry, mock label on the weather plugin, CLI demo output and `--help`.

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

Today the workflow has no macOS runner or DMG step yet; an unsigned-DMG `macos-latest` job is planned (TODO 16). No code signing.

## Status

First-batch tool scope, live weather/calendar backends, and the proactive “when to interrupt” policy are still open (`TODO.md` items 5–7).
