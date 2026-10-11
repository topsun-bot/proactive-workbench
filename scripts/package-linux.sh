#!/usr/bin/env bash
# Cross-compile a Linux amd64 CLI and pack it as a .tar.gz.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

VERSION="${APP_VERSION:-0.1.0}"
COMMIT="${GITHUB_SHA:-$(git rev-parse --short HEAD 2>/dev/null || echo unknown)}"
OUT_DIR="$ROOT/dist"
STAGE="$OUT_DIR/proactive-workbench-${VERSION}-linux-amd64"
TARBALL="$OUT_DIR/proactive-workbench-${VERSION}-linux-amd64.tar.gz"

rm -rf "$OUT_DIR"
mkdir -p "$STAGE"

echo "==> go version"
go version

echo "==> build linux/amd64"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath \
  -ldflags "-s -w -X main.version=${VERSION}" \
  -o "$STAGE/workbench" \
  ./cmd/workbench

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath \
  -ldflags "-s -w" \
  -o "$STAGE/proactivity" \
  ./cmd/proactivity

chmod +x "$STAGE/workbench" "$STAGE/proactivity"

cat > "$STAGE/README.txt" <<EOF
Proactive Workbench ${VERSION} (${COMMIT})
Linux amd64 CLI — shared Go core. Mac UI is a separate unsigned DMG.

Run the cross-tool demo (weather is MOCK; clear still creates reminders):

  ./workbench demo
  ./workbench demo --weather=clear
  ./workbench plan "bring an umbrella tomorrow 8am" --weather=rain
  ./workbench today --debug-fixture --now=2026-10-10T07:15:00+08:00

One proactivity tick against FIXTURE sources (no network):

  ./proactivity tick --debug-fixture --now=2026-10-10T15:00:00+08:00 --repeat=2

List plugins:

  ./workbench tools

Proactivity against FIXTURE sources (no network):

  ./proactivity tick --debug-fixture --now=2026-10-10T15:00:00+08:00 --repeat=2
  ./proactivity brief --debug-fixture --memory-fixture --now=2026-10-10T08:00:00+08:00
  ./proactivity tick --debug-fixture --json
  ./proactivity serve --listen=127.0.0.1:8741 --debug-fixture --memory-fixture
EOF

echo "==> archive"
tar -C "$OUT_DIR" -czf "$TARBALL" "$(basename "$STAGE")"

echo "==> artifact"
ls -lh "$TARBALL" "$STAGE/workbench"
# Record only the basename so sha256sum -c works after download.
(
	cd "$OUT_DIR"
	sha256sum "$(basename "$TARBALL")" | tee "$(basename "$TARBALL").sha256"
)
echo "TARBALL=$TARBALL"
