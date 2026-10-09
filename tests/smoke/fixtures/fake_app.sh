#!/usr/bin/env bash
# 仅供 selftest 使用的假产物，不是产品代码。模拟：常驻启动 / 崩溃 / 带伞示例输出。
case "${1:-}" in
  --serve)  echo "fake workbench started"; while true; do sleep 1; done ;;
  --crash)  echo "fake crash: boom" >&2; exit 3 ;;
  --demo)   echo "[日历] 明天 08:00 新建提醒"; echo "[天气] 明天降雨概率 80%（模拟数据）"; echo "[闹钟] 08:00 提醒：带伞" ;;
  --demo-bad) echo "[日历] nothing" ;;
  *) echo "usage: $0 --serve|--crash|--demo|--demo-bad" >&2; exit 2 ;;
esac
