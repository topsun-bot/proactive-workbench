# Contributing to Proactive Workbench

Thanks for your interest! Bug reports, docs fixes, new tools and core improvements are all welcome. Issues and PRs can be written in English or Chinese (中文也可以).

## Development setup

```bash
git clone https://github.com/topsun-bot/proactive-workbench.git
cd proactive-workbench
go test ./...          # Go 1.22+ on Linux; use Go 1.23+ on macOS (1.22 test binaries abort with "dyld missing LC_UUID")
./macos/test.sh        # Linux + macOS: client gate + EventKit-denied tests (extra Foundation tests run on macOS)
```

Useful entry points:

```bash
go run ./cmd/workbench demo --weather=rain
go run ./cmd/workbench today --weather=clear
go run ./cmd/proactivity tick
go run ./cmd/proactivity brief --memory-fixture
```

## Workflow

1. Open an issue first for anything bigger than a small fix, so we can agree on scope.
2. Fork, create a branch (`feat/...`, `fix/...`, `docs/...`), and keep each PR focused on one change.
3. Add or update tests. `go test ./...` must pass; CI runs on `ubuntu-latest` and `macos-latest`.
4. Run `gofmt` / `go vet ./...` before pushing.
5. Use [Conventional Commits](https://www.conventionalcommits.org/) style titles, e.g. `feat(tools): add unit conversion tool`.
6. Describe what changed, why, and how you tested it in the PR description.

## Adding a tool

Tools live under `internal/tools/<name>` and implement `tool.Tool`:

- `Descriptor()` returns id, name and summary.
- `Handle(Request) (Result, error)` handles a generic action + string payload.

Register the tool in the `tool.Registry`. Flows and the planner only talk to tools through the registry, so a live implementation registered under the same id replaces a MOCK one without changing the flows.

## Data and privacy rules

- Anything not backed by a live source must be clearly labeled `MOCK` or fixture — never present invented data as real.
- Do not enable location or other personal sensors by default; they must be opt-in.
- Never commit API keys, tokens, personal calendars or memory files.

## Reporting security issues

Please do **not** open a public issue for vulnerabilities — see [SECURITY.md](SECURITY.md).

## License

By contributing, you agree that your contributions are licensed under the [Apache License 2.0](LICENSE).
