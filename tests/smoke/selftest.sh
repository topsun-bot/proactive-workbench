#!/usr/bin/env bash
# 用假产物验证 run_smoke.sh 自身的判定逻辑（PASS/FAIL/SKIP 都要判对）。
set -u
D="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FAKE="$D/fixtures/fake_app.sh"
export PW_APP_ALIVE_SECS=2 PW_TIMEOUT_SECS=10 PW_SMOKE_LOG_DIR="$(mktemp -d)"
ok=0; bad=0
expect() { # 名称 期望退出码 期望出现的文本 -- env...
  local name="$1" want_rc="$2" want_txt="$3"; shift 3
  out="$(env -u PW_BUILD_CMD -u PW_APP_CMD -u PW_DEMO_CMD -u PW_SMOKE_STRICT "$@" bash "$D/run_smoke.sh" 2>&1)"; rc=$?
  if [ "$rc" = "$want_rc" ] && grep -Fq -- "$want_txt" <<<"$out"; then echo "selftest ok   : $name"; ok=$((ok+1))
  else echo "selftest FAIL : $name (rc=$rc, 期望 rc=$want_rc 且含「$want_txt」)"; echo "$out" | sed 's/^/    /'; bad=$((bad+1)); fi
}
expect "未对接时全部 SKIP 且退出 0" 0 "SKIP=3"
expect "strict 模式下 SKIP 判失败" 1 "PW_SMOKE_STRICT=1" PW_SMOKE_STRICT=1
expect "常驻产物启动通过" 0 "[PASS] app_starts" PW_BUILD_CMD=true PW_APP_CMD="$FAKE --serve"
expect "崩溃产物启动失败" 1 "[FAIL] app_starts" PW_BUILD_CMD=true PW_APP_CMD="$FAKE --crash"
expect "构建失败被捕获" 1 "[FAIL] build" PW_BUILD_CMD=false
expect "带伞示例通过" 0 "[PASS] umbrella_demo" PW_BUILD_CMD=true PW_DEMO_CMD="$FAKE --demo"
expect "带伞示例输出不符判失败" 1 "[FAIL] umbrella_demo" PW_BUILD_CMD=true PW_DEMO_CMD="$FAKE --demo-bad"
expect "exit0 模式" 0 "[PASS] app_starts" PW_BUILD_CMD=true PW_APP_START_MODE=exit0 PW_APP_CMD="$FAKE --demo"
echo "selftest: ok=$ok fail=$bad"
[ $bad -eq 0 ]
