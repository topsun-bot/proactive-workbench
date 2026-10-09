# Bug 复现包（Notion 开发看板 #13）

只放在 `tests/repro/`，不改产品代码。在 Linux 上从某个 commit 的 GitHub Actions 产物复现，并把日志打成可挂到 issue 的 tar.gz。

需要：已登录的 `gh`（`gh auth login`）、`jq`、`tar`。产物来自 `.github/workflows/linux-build.yml`（artifact `proactive-workbench-0.1.0-linux-amd64.tar.gz`）。

```bash
./tests/repro/run_artifact.sh <commit-sha>
# 例：
./tests/repro/run_artifact.sh c6d71d3
./tests/repro/run_artifact.sh HEAD --workflow "Linux build"
./tests/repro/run_artifact.sh HEAD -- plan "bring an umbrella tomorrow 8am" --weather=rain
```

脚本会：

1. 用 `gh run list --commit <sha>` 找该 commit 的 `Linux build` run（优先 conclusion=success）
2. `gh run download` 拉 artifact，解出 `workbench`
3. 按 CI 同样的命令跑：`version`、`tools`、雨天/晴天 `demo`（`--now=2026-10-09T13:00:00+08:00`）
4. 把 stdout/stderr、退出码、OS/kernel/arch、commit、run URL、时间戳写进 `tests/repro/.repro/<sha>-<utc>/`，并打出旁边的 `repro-<sha>-<utc>.tar.gz`

缺 run、缺产物、`gh` 未登录时以退出码 2 失败，并打印 `gh` 的原始错误。产物跑了但命令非 0 时仍打包，退出码 1。

把生成的 `.tar.gz` 挂到 GitHub issue，模板见 `.github/ISSUE_TEMPLATE/bug_report.md`。
