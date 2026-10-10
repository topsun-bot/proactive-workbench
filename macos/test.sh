#!/usr/bin/env bash
# Portable tests for Mac gate 2 and EventKit-denied calendar.Source fallback.
# Runs on ubuntu-latest and macos-latest. Does not link EventKit or Cocoa.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
SRC="$ROOT/TodayWorkbench"
OUT="$(mktemp -d)"
trap 'rm -rf "$OUT"' EXIT

cc -std=c11 -Wall -Wextra -Werror -I"$SRC" \
  -o "$OUT/gate_test" \
  "$SRC/gate.c" \
  "$SRC/gate_test.c"

echo "==> macos gate + EventKit-denied tests"
"$OUT/gate_test"

if [[ "$(uname -s)" == "Darwin" ]]; then
  echo "==> propose JSON parse (Foundation)"
  SDK="$(xcrun --show-sdk-path)"
  clang -fobjc-arc -std=c11 \
    -isysroot "$SDK" \
    -mmacosx-version-min=13.0 \
    -framework Foundation -framework Intents -framework UserNotifications \
    -I"$SRC" \
    -o "$OUT/propose_parse_test" \
    "$SRC/gate.c" \
    "$SRC/NotificationGate.m" \
    "$SRC/propose_parse_test.m"
  "$OUT/propose_parse_test"
fi
