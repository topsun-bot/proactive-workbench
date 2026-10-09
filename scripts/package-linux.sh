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

chmod +x "$STAGE/workbench"

cat > "$STAGE/README.txt" <<EOF
Proactive Workbench ${VERSION} (${COMMIT})
Linux amd64 CLI — no GUI, no macOS, no signing.

Run the cross-tool demo (weather is MOCK):

  ./workbench demo
  ./workbench demo --weather=clear
  ./workbench plan "bring an umbrella tomorrow 8am" --weather=rain

List plugins:

  ./workbench tools
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
