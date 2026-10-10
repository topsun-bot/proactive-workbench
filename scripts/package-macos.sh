#!/usr/bin/env bash
# Build an unsigned .app + DMG on macOS. Signing/notarization is out of scope.
set -euo pipefail

if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "package-macos.sh must run on macOS" >&2
  exit 1
fi

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

VERSION="${APP_VERSION:-0.1.0}"
COMMIT="${GITHUB_SHA:-$(git rev-parse --short HEAD 2>/dev/null || echo unknown)}"
OUT_DIR="$ROOT/dist"
APP_NAME="Today Workbench"
APP_DIR="$OUT_DIR/${APP_NAME}.app"
DMG="$OUT_DIR/proactive-workbench-${VERSION}-darwin-unsigned.dmg"

rm -rf "$OUT_DIR"
mkdir -p "$APP_DIR/Contents/MacOS" "$APP_DIR/Contents/Resources"

echo "==> go version"
go version

echo "==> build darwin workbench (shared Go core)"
CGO_ENABLED=0 go build -trimpath \
  -ldflags "-s -w -X main.version=${VERSION}" \
  -o "$APP_DIR/Contents/MacOS/workbench" \
  ./cmd/workbench
chmod +x "$APP_DIR/Contents/MacOS/workbench"

echo "==> compile AppKit + WKWebView + EventKit + Focus gate"
SDK="$(xcrun --show-sdk-path)"
clang -fobjc-arc \
  -isysroot "$SDK" \
  -mmacosx-version-min=13.0 \
  -framework Cocoa -framework WebKit \
  -framework EventKit -framework UserNotifications -framework Intents \
  -o "$APP_DIR/Contents/MacOS/TodayWorkbench" \
  "$ROOT/macos/TodayWorkbench/main.m" \
  "$ROOT/macos/TodayWorkbench/CalendarSource.m" \
  "$ROOT/macos/TodayWorkbench/NotificationGate.m" \
  "$ROOT/macos/TodayWorkbench/gate.c"
chmod +x "$APP_DIR/Contents/MacOS/TodayWorkbench"

cp "$ROOT/macos/TodayWorkbench/Info.plist" "$APP_DIR/Contents/Info.plist"

cat > "$APP_DIR/Contents/Resources/README.txt" <<EOF
Proactive Workbench ${VERSION} (${COMMIT})
Unsigned Mac app. The Today UI is served by the bundled Go core.

Default launch uses the real current time and does not inject MOCK weather.
Debug fixtures (canned 2026-10-10 07:15 + clear weather):
  PW_DEBUG_FIXTURE=1
  PW_DEBUG_NOW=2026-10-10T07:15:00+08:00   (optional)
  PW_DEBUG_WEATHER=clear                   (optional)
  or: workbench serve --debug-fixture

Listen address is 127.0.0.1:8741 (PR #3 API.md). If that port is busy the
core falls back and writes the address actually in use to:
  macOS:  ~/Library/Application Support/Today Workbench/port
  Linux:  \${XDG_CONFIG_HOME:-\$HOME/.config}/today-workbench/port
Override path: PW_PORT_FILE.

EventKit (Mac only) feeds calendar.Source-shaped events. Permission is
optional; denied access falls back to the MOCK schedule. No CoreLocation.
Banners (gate 2) require suggestion.propose == true and Focus off.
Missing propose is treated as false.

Signing and notarization are out of scope.
EOF

echo "==> create unsigned DMG"
hdiutil create -volname "Today Workbench" -srcfolder "$APP_DIR" -ov -format UDZO "$DMG"

echo "==> artifact"
ls -lh "$DMG" "$APP_DIR/Contents/MacOS/workbench" "$APP_DIR/Contents/MacOS/TodayWorkbench"
(
  cd "$OUT_DIR"
  shasum -a 256 "$(basename "$DMG")" | tee "$(basename "$DMG").sha256"
)
echo "DMG=$DMG"
