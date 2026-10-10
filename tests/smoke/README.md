# 冒烟与日常巡检测试（Notion 开发看板 #12）

```bash
bash tests/smoke/run_smoke.sh                 # 自动构建并巡检 build / app_starts / umbrella_demo
PW_SMOKE_STRICT=1 bash tests/smoke/run_smoke.sh
bash tests/smoke/selftest.sh                  # 验证 fake_app 注入场景 + 真实 Go 仓库四场景巡检
```

| 检查项 | 判定 |
|---|---|
| `build` | 自动探测 `go.mod` 并执行 `CGO_ENABLED=0 go build` 构建 `workbench` 与 `proactivity`（也可通过 `PW_BUILD_CMD` 覆盖） |
| `app_starts` | 验证 `workbench version` 退出 0，且 `workbench serve --addr=127.0.0.1:0` 启动后存活 `PW_APP_ALIVE_SECS`（默认 5s）不崩溃 |
| `umbrella_demo` | 依次验证 4 个巡检场景并记录日志（含 commit、场景模式、期望与实际输出）：<br>1. `demo --weather=rain`：`Reminders created`、`Outcome:  rain`、`今天可能下雨`、`降水概率 80%`<br>2. `demo --weather=clear`：`Reminders created`、`Outcome:  no_rain`、`今天降水概率 5%`、`带不带你定`，且不含 `Reminders skipped`<br>3. `demo --weather=unavailable`：`Reminders created`、`Outcome:  weather_unavailable`、`记得带伞（天气暂时查不到）。`，且不伪造降水概率<br>4. `today --weather=unavailable`：包含 `Weather unavailable (no MOCK scenario)`，不伪造 `Clear · 22°C` |

在无 `go.mod` 且未配置环境变量的空目录中记为 **SKIP（待对接）**；`PW_SMOKE_STRICT=1` 时任何 SKIP 均判为失败。
