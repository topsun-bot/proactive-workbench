# Linux 冒烟测试（Notion 开发看板 #12）

只放在 `tests/`，不改产品代码。

```bash
bash tests/smoke/run_smoke.sh   # 冒烟：build / app_starts / umbrella_demo
bash tests/smoke/selftest.sh    # 用 fixtures/fake_app.sh 验证脚本自身的判定逻辑
```

| 检查项 | 判定 |
|---|---|
| build | `PW_BUILD_CMD`（或自动探测 Package.swift→`swift build`、package.json、Makefile）退出 0 |
| app_starts | `PW_APP_CMD` 启动后存活 `PW_APP_ALIVE_SECS`（默认 5s）不崩；或 `PW_APP_START_MODE=exit0` 时以 0 退出 |
| umbrella_demo | `PW_DEMO_CMD` 超时内退出 0，输出命中 `PW_DEMO_EXPECT`（默认：`带伞\|umbrella`、`08:00\|8:00\|八点`、`模拟\|mock`） |

未配置的项记为 **SKIP（待对接）**，不算失败；`PW_SMOKE_STRICT=1` 时 SKIP 也算失败。

## 待对接（TODO）

产品框架（TODO.md #2 主窗口+插件接口、#3 macOS 构建出 DMG、#4 带伞示例，负责人 Shaoruru）尚未合入 main，需要：

1. **Linux 可执行入口**：产品是 Mac 客户端（DMG），Linux 上无法启动 .app。需要框架提供可在 Linux 构建运行的无头入口（如 SwiftPM 的核心库 + CLI target，`--headless` / `--self-check`），然后设置 `PW_BUILD_CMD`、`PW_APP_CMD`。
2. **带伞示例的命令行入口与输出约定**：需要 `PW_DEMO_CMD`（如 `<cli> demo umbrella`），并确认输出里包含「带伞」、「08:00」、「模拟数据」标注；若约定不同，改 `PW_DEMO_EXPECT`。
3. **DMG 产物本身的启动验证**只能在 macOS 上做，不在本 Linux 冒烟范围内。
4. 定时巡检暂不开启。
