---
name: Bug report
about: 报告 Linux 上可复现的问题 / Report a reproducible Linux bug
title: "[bug] "
labels: bug
---

## 复现步骤 / Steps to reproduce

1.
2.
3.

建议先用 CI 产物在 Linux 上复现（不要只贴本地未提交改动）：

```bash
./tests/repro/run_artifact.sh <commit-sha>
```

`--workflow` / `--artifact` / `--run-id` 见 `tests/repro/README.md`。`--` 后面的参数会传给解压出的 `workbench`。

## 期望行为 / Expected behavior



## 实际行为 / Actual behavior



## 日志 / Logs

把 `./tests/repro/run_artifact.sh` 打出的 **bundle**（默认 `tests/repro/.repro/<sha>-<utc>.tar.gz`）挂到本 issue。

包内应有：

- `environment.txt` — OS / kernel / arch、commit SHA、Actions run URL、时间戳
- `summary.txt` / `commands.tsv` — 每条命令和退出码
- `logs/*.log` — `workbench version` / `tools` / 雨天与晴天 `demo` 的 stdout+stderr
- `run.json` — 用到的 GitHub Actions run

若脚本在下载阶段就失败（没有该 commit 的 run、没有 artifact、`gh` 未登录），把终端里的 **ERROR:** 原文贴在这里，不要改写。

## Commit SHA

- SHA（40 位或 `git rev-parse HEAD`）：
- Actions run URL（脚本 `environment.txt` 里的 `run_url=`，没有就留空）：
