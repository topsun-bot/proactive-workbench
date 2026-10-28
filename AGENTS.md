# AGENTS.md

Guidance for coding agents (Codex, Cursor, etc.) working in this repository.

## Project

Go module `github.com/topsun-bot/proactive-workbench`: a proactive agent core
(sense → goal → plan → interrupt gate) with a Linux CLI (`cmd/workbench`,
`cmd/proactivity`) and a macOS AppKit/WKWebView client (`macos/TodayWorkbench`).

## Commands

- Test: `go test ./...` (must pass before every commit)
- Vet/format: `go vet ./...`, `gofmt -l .` (files you touch must be gofmt-clean)
- Client gate tests: `./macos/test.sh` (runs on Linux and macOS; Foundation tests only on macOS)
- Demo: `go run ./cmd/workbench demo --weather=rain|clear`
- Package: `./scripts/package-linux.sh`, `./scripts/package-macos.sh`

## Rules

- Flows and the planner call tools only through `tool.Registry`; do not import tool packages directly from flows.
- Keep data honest: un-wired sources stay labeled `MOCK` / fixture; never default to fabricated "good" values (e.g. clear weather when weather is unavailable).
- No new personal sensors (location, contacts, etc.) enabled by default.
- Keep PRs small and focused, with tests; update `README.md` when behavior or commands change.
- Do not commit secrets, personal calendars or memory files.

## Review checklist

When reviewing a PR, check: tests added/updated, `MOCK` labeling preserved,
`127.0.0.1` default bind unchanged, no new network calls without a timeout,
and README/TODO kept in sync.
