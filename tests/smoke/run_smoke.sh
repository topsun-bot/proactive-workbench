#!/usr/bin/env bash
# proactive-workbench Linux 冒烟测试（Notion 开发看板 #12）
#
# 检查两件事：
#   1. app_starts      —— 构建产物能启动（启动后在超时内不崩溃，或 --version/--help 正常退出）
#   2. umbrella_demo   —— 「明早八点提醒带伞」示例（日历+天气+闹钟，模拟天气）能端到端跑通
#
# 产品框架（TODO.md #2/#3/#4，Shaoruru）尚未合入 main，因此本脚本通过环境变量对接，
# 未配置/未检测到时该项记为 SKIP（待对接），不算失败；设 PW_SMOKE_STRICT=1 时 SKIP 也算失败。
#
# 对接用环境变量（全部可选）：
#   PW_BUILD_CMD          构建命令，在仓库根目录执行（如 "swift build -c release"）
#   PW_APP_CMD            启动构建产物的命令（如 ".build/release/ProactiveWorkbench --headless"）
#   PW_APP_START_MODE     "alive"（默认：启动后存活 PW_APP_ALIVE_SECS 秒即通过）
#                         或 "exit0"（命令需在超时内以 0 退出，适合 --version / --self-check）
#   PW_APP_ALIVE_SECS     存活判定秒数，默认 5
#   PW_DEMO_CMD           跑带伞示例的命令（需在超时内退出并把结果打到 stdout/stderr）
#   PW_DEMO_EXPECT        示例输出必须匹配的正则（grep -E，用 ";;" 分隔多个），
#                         默认 "带伞|umbrella;;(08:00|8:00|八点);;模拟|mock"
#   PW_TIMEOUT_SECS       单项超时，默认 60
#   PW_SMOKE_STRICT       设为 1 时 SKIP 视为失败
#   PW_SMOKE_LOG_DIR      日志目录，默认 tests/smoke/.logs
#
# 退出码：0 = 无 FAIL；1 = 有 FAIL（或 strict 下有 SKIP）；2 = 用法/环境错误
set -u

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="${PW_REPO_ROOT:-$(cd "$SCRIPT_DIR/../.." && pwd)}"
LOG_DIR="${PW_SMOKE_LOG_DIR:-$SCRIPT_DIR/.logs}"
TIMEOUT="${PW_TIMEOUT_SECS:-60}"
ALIVE_SECS="${PW_APP_ALIVE_SECS:-5}"
START_MODE="${PW_APP_START_MODE:-alive}"
DEMO_EXPECT="${PW_DEMO_EXPECT:-带伞|umbrella;;(08:00|8:00|八点);;模拟|mock}"
STRICT="${PW_SMOKE_STRICT:-0}"

command -v timeout >/dev/null 2>&1 || { echo "ERROR: 需要 coreutils timeout" >&2; exit 2; }
mkdir -p "$LOG_DIR"

PASS=0; FAIL=0; SKIP=0
declare -a RESULTS=()
record() { # status name detail
  RESULTS+=("$1  $2  $3")
  case "$1" in PASS) PASS=$((PASS+1));; FAIL) FAIL=$((FAIL+1));; SKIP) SKIP=$((SKIP+1));; esac
  echo "[$1] $2 — $3"
}
show_log_tail() { echo "  ---- $1 (末尾 20 行) ----"; tail -n 20 "$1" | sed 's/^/  | /'; }

# ---------- 自动探测（框架合入后可直接生效，探测不到则走 SKIP） ----------
detect_build_cmd() {
  [ -n "${PW_BUILD_CMD:-}" ] && { echo "$PW_BUILD_CMD"; return; }
  if [ -f "$REPO_ROOT/Package.swift" ] && command -v swift >/dev/null 2>&1; then echo "swift build"; return; fi
  if [ -f "$REPO_ROOT/package.json" ] && command -v npm >/dev/null 2>&1; then echo "npm ci && npm run build --if-present"; return; fi
  if [ -f "$REPO_ROOT/Makefile" ]; then echo "make"; return; fi
  echo ""
}

echo "== proactive-workbench smoke =="
echo "repo:   $REPO_ROOT"
echo "commit: $(git -C "$REPO_ROOT" rev-parse --short HEAD 2>/dev/null || echo n/a)"
echo "host:   $(uname -srm)"
echo "time:   $(date '+%F %T %Z')"
echo

# ---------- 0. 构建 ----------
BUILD_CMD="$(detect_build_cmd)"
BUILD_OK=1
if [ -z "$BUILD_CMD" ]; then
  if [ -f "$REPO_ROOT/Package.swift" ]; then
    record SKIP build "待对接：检测到 Package.swift 但本机无 swift 工具链；请安装 Swift for Linux 或设置 PW_BUILD_CMD"
  else
    record SKIP build "待对接：仓库中未找到构建入口（Package.swift / package.json / Makefile），且未设置 PW_BUILD_CMD"
  fi
else
  ( cd "$REPO_ROOT" && timeout "$TIMEOUT" bash -c "$BUILD_CMD" ) >"$LOG_DIR/build.log" 2>&1
  rc=$?
  if [ $rc -eq 0 ]; then record PASS build "$BUILD_CMD"
  else BUILD_OK=0; record FAIL build "\"$BUILD_CMD\" 退出码 $rc（124=超时）"; show_log_tail "$LOG_DIR/build.log"; fi
fi

# ---------- 1. 构建产物能启动 ----------
if [ -z "${PW_APP_CMD:-}" ]; then
  record SKIP app_starts "待对接：未设置 PW_APP_CMD（产品尚无可在 Linux 上启动的产物/无头入口，见 TODO.md #2/#3）"
elif [ $BUILD_OK -eq 0 ]; then
  record FAIL app_starts "构建失败，未尝试启动"
else
  log="$LOG_DIR/app_starts.log"
  if [ "$START_MODE" = "exit0" ]; then
    ( cd "$REPO_ROOT" && timeout "$TIMEOUT" bash -c "$PW_APP_CMD" ) >"$log" 2>&1; rc=$?
    if [ $rc -eq 0 ]; then record PASS app_starts "\"$PW_APP_CMD\" 以 0 退出"
    else record FAIL app_starts "\"$PW_APP_CMD\" 退出码 $rc（124=超时）"; show_log_tail "$log"; fi
  else
    ( cd "$REPO_ROOT" && exec bash -c "exec $PW_APP_CMD" ) >"$log" 2>&1 &
    pid=$!
    sleep "$ALIVE_SECS"
    if kill -0 "$pid" 2>/dev/null; then
      record PASS app_starts "\"$PW_APP_CMD\" 启动后 ${ALIVE_SECS}s 仍在运行"
      kill "$pid" 2>/dev/null; sleep 1; kill -9 "$pid" 2>/dev/null; wait "$pid" 2>/dev/null
    else
      wait "$pid"; rc=$?
      record FAIL app_starts "\"$PW_APP_CMD\" 在 ${ALIVE_SECS}s 内退出，退出码 $rc"; show_log_tail "$log"
    fi
  fi
fi

# ---------- 2. 带伞示例端到端 ----------
if [ -z "${PW_DEMO_CMD:-}" ]; then
  record SKIP umbrella_demo "待对接：未设置 PW_DEMO_CMD（「明早八点提醒带伞」示例尚未合入 main，见 TODO.md #4；输出约定需与 Shaoruru 确认）"
elif [ $BUILD_OK -eq 0 ]; then
  record FAIL umbrella_demo "构建失败，未尝试运行示例"
else
  log="$LOG_DIR/umbrella_demo.log"
  ( cd "$REPO_ROOT" && timeout "$TIMEOUT" bash -c "$PW_DEMO_CMD" ) >"$log" 2>&1; rc=$?
  if [ $rc -ne 0 ]; then
    record FAIL umbrella_demo "\"$PW_DEMO_CMD\" 退出码 $rc（124=超时）"; show_log_tail "$log"
  else
    missing=""
    IFS=$'\n' read -r -d '' -a pats < <(printf '%s' "$DEMO_EXPECT" | sed 's/;;/\n/g'; printf '\0')
    for p in "${pats[@]}"; do
      [ -z "$p" ] && continue
      grep -Eq -- "$p" "$log" || missing="$missing [$p]"
    done
    if [ -z "$missing" ]; then record PASS umbrella_demo "退出码 0，输出命中全部期望：$DEMO_EXPECT"
    else record FAIL umbrella_demo "退出码 0，但输出未命中:$missing"; show_log_tail "$log"; fi
  fi
fi

echo
echo "== 汇总: PASS=$PASS FAIL=$FAIL SKIP=$SKIP (日志: $LOG_DIR) =="
if [ $FAIL -gt 0 ]; then exit 1; fi
if [ "$STRICT" = "1" ] && [ $SKIP -gt 0 ]; then echo "PW_SMOKE_STRICT=1：存在 SKIP，判为失败"; exit 1; fi
exit 0
