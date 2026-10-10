#!/usr/bin/env bash
# proactive-workbench 冒烟与日常巡检测试（Notion 开发看板 #12）
#
# 检查三件事：
#   0. build          —— 构建 Go 产物（workbench + proactivity）或自定义 PW_BUILD_CMD
#   1. app_starts     —— 构建产物能启动（version 正常退出 + serve 在超时内稳定存活不崩溃）
#   2. umbrella_demo  —— 「明早八点提醒带伞」示例（rain / clear / unavailable 三场景）及
#                        today 未知天气诚实降级（不伪造 Clear）端到端验证
#
# 对接用环境变量（全部可选）：
#   PW_BUILD_CMD          构建命令，在仓库根目录执行（未设置且存在 go.mod 时自动构建 Go 产物）
#   PW_APP_CMD            启动构建产物的命令
#   PW_APP_START_MODE     "alive"（默认：启动后存活 PW_APP_ALIVE_SECS 秒即通过）
#                         或 "exit0"（命令需在超时内以 0 退出，适合 --version / --self-check）
#   PW_APP_ALIVE_SECS     存活判定秒数，默认 5
#   PW_DEMO_CMD           跑带伞示例的命令（未设置且使用默认 Go 构建时自动跑完整 4 场景巡检）
#   PW_DEMO_EXPECT        自定义 PW_DEMO_CMD 时输出必须匹配的正则（grep -E，用 ";;" 分隔多个）
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
DEMO_EXPECT="${PW_DEMO_EXPECT:-带伞|umbrella;;(08:00|8:00|八点);;模拟|mock|MOCK}"
STRICT="${PW_SMOKE_STRICT:-0}"

if ! command -v timeout >/dev/null 2>&1; then
  if command -v gtimeout >/dev/null 2>&1; then
    timeout() { gtimeout "$@"; }
  elif command -v perl >/dev/null 2>&1; then
    timeout() {
      perl -e '
        my $secs = shift @ARGV;
        my $pid = fork();
        die "fork: $!" unless defined $pid;
        if ($pid == 0) { setpgrp(0, 0); exec @ARGV; exit 127; }
        local $SIG{ALRM} = sub { kill -9, -$pid; kill -9, $pid; waitpid($pid, 0); exit 124; };
        alarm($secs);
        waitpid($pid, 0);
        my $st = $?;
        alarm(0);
        if ($st & 127) { exit(128 + ($st & 127)); }
        exit($st >> 8);
      ' -- "$@"
    }
  else
    echo "ERROR: 需要 coreutils timeout 或 perl" >&2
    exit 2
  fi
fi

mkdir -p "$LOG_DIR"
BIN_DIR="$LOG_DIR/bin"
mkdir -p "$BIN_DIR"

PASS=0; FAIL=0; SKIP=0
declare -a RESULTS=()
record() { # status name detail
  RESULTS+=("$1  $2  $3")
  case "$1" in PASS) PASS=$((PASS+1));; FAIL) FAIL=$((FAIL+1));; SKIP) SKIP=$((SKIP+1));; esac
  echo "[$1] $2 — $3"
}
show_log_tail() { echo "  ---- $1 (末尾 25 行) ----"; tail -n 25 "$1" | sed 's/^/  | /'; }

AUTO_GO=0
if [ -z "${PW_BUILD_CMD:-}" ] && [ -f "$REPO_ROOT/go.mod" ] && command -v go >/dev/null 2>&1; then
  AUTO_GO=1
fi

# ---------- 自动探测 ----------
detect_build_cmd() {
  [ -n "${PW_BUILD_CMD:-}" ] && { echo "$PW_BUILD_CMD"; return; }
  if [ "$AUTO_GO" -eq 1 ]; then
    echo "CGO_ENABLED=0 go build -o \"$BIN_DIR/workbench\" ./cmd/workbench && CGO_ENABLED=0 go build -o \"$BIN_DIR/proactivity\" ./cmd/proactivity"
    return
  fi
  if [ -f "$REPO_ROOT/Package.swift" ] && command -v swift >/dev/null 2>&1; then echo "swift build"; return; fi
  if [ -f "$REPO_ROOT/package.json" ] && command -v npm >/dev/null 2>&1; then echo "npm ci && npm run build --if-present"; return; fi
  if [ -f "$REPO_ROOT/Makefile" ]; then echo "make"; return; fi
  echo ""
}

COMMIT_SHA="$(git -C "$REPO_ROOT" rev-parse --short HEAD 2>/dev/null || echo n/a)"
echo "== proactive-workbench smoke =="
echo "repo:   $REPO_ROOT"
echo "commit: $COMMIT_SHA"
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
    record SKIP build "待对接：仓库中未找到构建入口（go.mod / Package.swift / package.json / Makefile），且未设置 PW_BUILD_CMD"
  fi
else
  ( cd "$REPO_ROOT" && timeout "$TIMEOUT" bash -c "$BUILD_CMD" ) >"$LOG_DIR/build.log" 2>&1
  rc=$?
  if [ $rc -eq 0 ]; then record PASS build "$BUILD_CMD (commit=$COMMIT_SHA)"
  else BUILD_OK=0; record FAIL build "\"$BUILD_CMD\" 退出码 ${rc}（124=超时）"; show_log_tail "$LOG_DIR/build.log"; fi
fi

# ---------- 1. 构建产物能启动 ----------
APP_CMD="${PW_APP_CMD:-}"
if [ -z "$APP_CMD" ] && [ "$AUTO_GO" -eq 1 ]; then
  APP_CMD="\"$BIN_DIR/workbench\" serve --addr=127.0.0.1:0"
fi

if [ -z "$APP_CMD" ]; then
  record SKIP app_starts "待对接：未设置 PW_APP_CMD"
elif [ $BUILD_OK -eq 0 ]; then
  record FAIL app_starts "构建失败，未尝试启动"
else
  log="$LOG_DIR/app_starts.log"
  : >"$log"
  if [ "$AUTO_GO" -eq 1 ] && [ -z "${PW_APP_CMD:-}" ]; then
    "$BIN_DIR/workbench" version >>"$log" 2>&1
    vrc=$?
    if [ $vrc -ne 0 ]; then
      record FAIL app_starts "workbench version 退出码 $vrc"
      show_log_tail "$log"
    else
      ( cd "$REPO_ROOT" && PW_PORT_FILE="$LOG_DIR/port" exec bash -c "exec $APP_CMD" ) >>"$log" 2>&1 &
      pid=$!
      sleep "$ALIVE_SECS"
      if kill -0 "$pid" 2>/dev/null; then
        record PASS app_starts "workbench version 正常退出 0 且 serve 启动后 ${ALIVE_SECS}s 仍在运行"
        kill "$pid" 2>/dev/null; sleep 1; kill -9 "$pid" 2>/dev/null; wait "$pid" 2>/dev/null
      else
        wait "$pid"; rc=$?
        record FAIL app_starts "\"$APP_CMD\" 在 ${ALIVE_SECS}s 内退出，退出码 $rc"; show_log_tail "$log"
      fi
    fi
  elif [ "$START_MODE" = "exit0" ]; then
    ( cd "$REPO_ROOT" && timeout "$TIMEOUT" bash -c "$APP_CMD" ) >"$log" 2>&1; rc=$?
    if [ $rc -eq 0 ]; then record PASS app_starts "\"$APP_CMD\" 以 0 退出"
    else record FAIL app_starts "\"$APP_CMD\" 退出码 ${rc}（124=超时）"; show_log_tail "$log"; fi
  else
    ( cd "$REPO_ROOT" && exec bash -c "exec $APP_CMD" ) >"$log" 2>&1 &
    pid=$!
    sleep "$ALIVE_SECS"
    if kill -0 "$pid" 2>/dev/null; then
      record PASS app_starts "\"$APP_CMD\" 启动后 ${ALIVE_SECS}s 仍在运行"
      kill "$pid" 2>/dev/null; sleep 1; kill -9 "$pid" 2>/dev/null; wait "$pid" 2>/dev/null
    else
      wait "$pid"; rc=$?
      record FAIL app_starts "\"$APP_CMD\" 在 ${ALIVE_SECS}s 内退出，退出码 $rc"; show_log_tail "$log"
    fi
  fi
fi

# ---------- 2. 带伞示例端到端与多场景巡检 ----------
run_go_multi_scenario_demo() {
  local log="$LOG_DIR/umbrella_demo.log"
  : >"$log"
  local wb="$BIN_DIR/workbench"
  local failed_detail=""

  check_scenario() {
    local mode_name="$1"
    local cmd_str="$2"
    local must_match="$3"
    local must_not_match="$4"
    local slog="$LOG_DIR/demo_${mode_name}.log"

    {
      echo "=== [SCENARIO: $mode_name] commit=$COMMIT_SHA ==="
      echo "CMD: $cmd_str"
      echo "EXPECT_MATCH: $must_match"
      [ -n "$must_not_match" ] && echo "EXPECT_NOT_MATCH: $must_not_match"
    } >>"$log"

    ( cd "$REPO_ROOT" && timeout "$TIMEOUT" bash -c "$cmd_str" ) >"$slog" 2>&1
    local rc=$?
    cat "$slog" >>"$log"
    echo "" >>"$log"

    if [ $rc -ne 0 ]; then
      failed_detail="${failed_detail} [${mode_name}: 退出码 ${rc}]"
      return
    fi

    IFS=$'\n' read -r -d '' -a pats < <(printf '%s' "$must_match" | sed 's/;;/\n/g'; printf '\0')
    for p in "${pats[@]}"; do
      [ -z "$p" ] && continue
      if ! grep -Eq -- "$p" "$slog"; then
        failed_detail="${failed_detail} [${mode_name} 缺少期望: ${p}]"
      fi
    done

    if [ -n "$must_not_match" ]; then
      IFS=$'\n' read -r -d '' -a npats < <(printf '%s' "$must_not_match" | sed 's/;;/\n/g'; printf '\0')
      for np in "${npats[@]}"; do
        [ -z "$np" ] && continue
        if grep -Eq -- "$np" "$slog"; then
          failed_detail="${failed_detail} [${mode_name} 出现禁止文本: ${np}]"
        fi
      done
    fi
  }

  check_scenario "rain" \
    "\"$wb\" demo --weather=rain --tz=Asia/Shanghai --now=2026-10-09T13:00:00+08:00" \
    "Reminders created;;Outcome:  rain;;今天可能下雨;;降水概率 80%;;记得带伞;;MOCK weather \\(not live data\\);;2026-10-10 08:00" \
    ""

  check_scenario "clear" \
    "\"$wb\" demo --weather=clear --tz=Asia/Shanghai --now=2026-10-09T13:00:00+08:00" \
    "Reminders created;;Outcome:  no_rain;;今天降水概率 5%;;带不带你定;;MOCK weather \\(not live data\\);;2026-10-10 08:00" \
    "Reminders skipped"

  check_scenario "unavailable" \
    "\"$wb\" demo --weather=unavailable --tz=Asia/Shanghai --now=2026-10-09T13:00:00+08:00" \
    "Reminders created;;Outcome:  weather_unavailable;;记得带伞（天气暂时查不到）。;;2026-10-10 08:00" \
    "今天降水概率"

  check_scenario "today_unavailable" \
    "\"$wb\" today --weather=unavailable --tz=Asia/Shanghai --now=2026-10-10T07:15:00+08:00" \
    "Weather unavailable \\(no MOCK scenario\\)" \
    "Clear · 22°C"

  if [ -z "$failed_detail" ]; then
    record PASS umbrella_demo "rain / clear / unavailable / today_unavailable 四场景全部命中期望 (commit=$COMMIT_SHA)"
  else
    record FAIL umbrella_demo "多场景巡检未通过:${failed_detail}"
    show_log_tail "$log"
  fi
}

if [ -z "${PW_DEMO_CMD:-}" ] && [ "$AUTO_GO" -eq 1 ]; then
  if [ $BUILD_OK -eq 0 ]; then
    record FAIL umbrella_demo "构建失败，未尝试运行示例"
  else
    run_go_multi_scenario_demo
  fi
elif [ -z "${PW_DEMO_CMD:-}" ]; then
  record SKIP umbrella_demo "待对接：未设置 PW_DEMO_CMD"
elif [ $BUILD_OK -eq 0 ]; then
  record FAIL umbrella_demo "构建失败，未尝试运行示例"
else
  log="$LOG_DIR/umbrella_demo.log"
  ( cd "$REPO_ROOT" && timeout "$TIMEOUT" bash -c "$PW_DEMO_CMD" ) >"$log" 2>&1; rc=$?
  if [ $rc -ne 0 ]; then
    record FAIL umbrella_demo "\"$PW_DEMO_CMD\" 退出码 ${rc}（124=超时）"; show_log_tail "$log"
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
