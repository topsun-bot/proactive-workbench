# 跨平台主动性工作台（Linux 优先）— 开发 To-do

状态：待办 / 进行中 / 已完成 / 搁置。负责人为张益新指定的 bot 或本人。最后更新：2026-10-09。

> 变更（2026-10-09，张益新决定）：短期内找不到 Mac，开发与交付改为 Linux，不再出 DMG。Mac 相关事项（8、9）搁置。
> 原第 10（PW 项目经理）、11（PW 发布验收 QA）、14（PW 界面评审）项已按张益新决定移出计划。
>
> 统一规则：负责人自己开工；开始时把状态改为进行中，完成后改为已完成并写明完成了什么和 commit 链接。

方向已改为 **Linux CLI**（无可用 Mac；不做 DMG / 签名）。实现见 `cursor/linux-workbench-cli-d1a9`。

| # | 任务 | 状态 | 负责人 | 验收标准 |
|---|---|---|---|---|
| 1 | 建仓库 topsun-bot/proactive-workbench（已公开） | 已完成 | 主动Agent | 仓库可访问，Public，默认分支 main |
| 2 | Linux 客户端框架（桌面应用或命令行，形态由 Shaoruru 定；主界面 + 工具插件接口） | 进行中 | Shaoruru | Go CLI `workbench`；插件接口 + calendar/weather/alarm 桩；PR #1 |
| 3 | GitHub Actions 改用 ubuntu-latest 构建 Linux 可运行产物（去掉 macOS 构建机与 DMG） | 进行中 | Shaoruru | ubuntu-latest 上 Actions 跑绿，tar.gz artifact；workflow `Linux build` |
| 4 | 「明天早上八点提醒带伞」跨工具演示在 Linux 上跑通（天气先 mock） | 进行中 | Shaoruru | `workbench demo`；雨天建日历+闹钟，晴天跳过；CI 日志打印输出 |
| 5 | 首版范围：选 3–5 个工具，一页 PRD | 待办 | A3 产品经理 | 一页 PRD，张益新批准 |
| 6 | 数据源：天气 API、日历/提醒权限 | 已完成 | A4 技术互联 | Open-Meteo（无 key）接口+fixture mock+注入 HTTP 适配器；Linux 日历推荐本地 ICS，CalDAV 为不联网的 stub；设计见 `internal/datasources/DESIGN.md`。[cf6ef92](https://github.com/topsun-bot/proactive-workbench/commit/cf6ef92cc45688cc0ea73499c115cc90e532bb36) |
| 7 | 主动性内核设计：感知 → 目标 → 规划 → 何时打扰 | 已完成 | A4 技术互联 | `internal/proactivity` + `cmd/proactivity tick`；打扰策略：分数阈值、安静时段、相同 fingerprint 不重复打扰。带伞目标由雨天 fixture 生成（不改 Shaoruru umbrella 演示）。[cf6ef92](https://github.com/topsun-bot/proactive-workbench/commit/cf6ef92cc45688cc0ea73499c115cc90e532bb36) |
| 8 | Apple 开发者证书/公证 是否购买 | 搁置 | 张益新 | 短期无 Mac，搁置（2026-10-09） |
| 9 | 在 Mac 上安装 DMG 并反馈 | 搁置 | 张益新 | 短期无 Mac，搁置（2026-10-09） |
| 12 | 工作日巡检：构建、Linux 产物、带伞演示是否退化 | 待办 | PW 日常巡检 QA | 直接在 Linux 上验收，无需 Mac；巡检脚本提交到 tests/（附 commit 链接）；确认开启后每个工作日一份报告 |
| 13 | Bug 复现包（步骤、日志、截图、提交） | 待办 | PW Bug 复现 | 直接在 Linux 上复现，无需 Mac；复现脚本提交到 tests/（附 commit 链接） |
