# `internal/connector` — Unified Connector Contract, Discovery & Incremental Sync

`internal/connector` provides the cross-platform (macOS + Linux, `CGO_ENABLED=0` pure Go) foundation for connecting external personal data sources across the seven workbench domains:

- `calendar` (`KindCalendar`)
- `notes` (`KindNotes`)
- `reminders` (`KindReminders`)
- `files` (`KindFiles`)
- `contacts` (`KindContacts`)
- `mail` (`KindMail`)
- `apps` (`KindApps`)

## Core Guarantees

1. **Unified Capability Manifest & Least Privilege (`Descriptor`, `PermissionScope`)**:
   - Every connector declares its `Mode` (`mock` vs `live`), `SupportedKinds`, `SupportedActions` (`read`, `list`, `sync`, `create`, `update`, `delete`), `RetryPolicy`, and scoped `Permissions`.
   - Broad wildcard scopes (`*`, `/`, `full_disk_access`, `all_accounts`, `all_files`) or undeclared kinds/actions are rejected at registration time with `CodePermissionViolation`.
2. **Mandatory Provenance & Honest MOCK Labeling (`Object`, `ValidateObject`)**:
   - Every external record must carry `Source`, `ExternalID`, `Kind`, `UpdatedAt`, `Version`, `Timezone` (validated IANA location), and `IsMock`.
   - `ModeMock` connectors cannot emit `IsMock=false` objects, and `RequireLive=true` sync/list/write calls reject any `ModeMock` connector or `IsMock=true` object with `CodeMockAsLive`.
3. **Extensible Discovery & Idempotent Incremental Sync (`Registry`, `Engine`)**:
   - New data sources implement `Connector` and register with `Registry.Register(c)` without changing `internal/memory` or `internal/proactivity`.
   - `Engine.Sync` and `Engine.Write` deduplicate objects by canonical ID (`<kind>:<external_id>`) + `Version` and track `IdempotencyKey` ledgers so repeated syncs or retried writes never create duplicate objects.
4. **Diagnosable Structured Errors (`*Error`, `ErrorCode`)**:
   - Disconnections (`CodeDisconnected`), rate limits (`CodeRateLimited` with `RetryAfter`), missing fields (`CodeMissingField` with `Field`), and invalid timezones (`CodeInvalidTimezone`) return structured `*Error` values inspectable via `IsCode(err, code)`.
5. **Local Contract Test Adapters (`MemoryAdapter`, `SnapshotAdapter`)**:
   - Two distinct local adapters (`MemoryAdapter` for in-memory structs and `SnapshotAdapter` for flat serialized JSON/export snapshots) verify semantic equivalence across all seven kinds in `contract_test.go`.
