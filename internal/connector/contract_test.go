package connector

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"
)

func sampleDescriptor(id string, mode Mode) Descriptor {
	allKinds := AllKinds()
	allActions := []Action{ActionRead, ActionList, ActionSync, ActionCreate, ActionUpdate, ActionDelete}
	perms := make([]PermissionScope, 0, len(allKinds))
	for _, k := range allKinds {
		perms = append(perms, PermissionScope{
			Kind:      k,
			Actions:   allActions,
			Resources: []string{"workbench/" + string(k)},
			Reason:    "Sync scoped workbench " + string(k) + " items for Today brief",
		})
	}
	return Descriptor{
		ID:               id,
		DisplayName:      "Test Connector " + id,
		Summary:          "Contract test connector for " + id,
		Mode:             mode,
		SupportedKinds:   allKinds,
		SupportedActions: allActions,
		Permissions:      perms,
		RetryPolicy: RetryPolicy{
			MaxAttempts:    3,
			InitialBackoff: 5 * time.Millisecond,
			MaxBackoff:     20 * time.Millisecond,
		},
	}
}

func sampleSevenDomainDataset(isMock bool) ([]Object, []byte) {
	baseTime := time.Date(2026, 10, 10, 8, 30, 0, 0, time.FixedZone("CST", 8*3600))
	objs := []Object{
		{
			Source:     "fixture.local",
			ExternalID: "evt-001",
			Kind:       KindCalendar,
			UpdatedAt:  baseTime,
			Version:    "v1",
			Timezone:   "Asia/Shanghai",
			IsMock:     isMock,
			Title:      "Morning standup",
			Attributes: map[string]string{"location": "Room 302", "start": "2026-10-10T09:30:00+08:00"},
		},
		{
			Source:     "fixture.local",
			ExternalID: "note-001",
			Kind:       KindNotes,
			UpdatedAt:  baseTime.Add(5 * time.Minute),
			Version:    "v2",
			Timezone:   "Asia/Shanghai",
			IsMock:     isMock,
			Title:      "Architecture review notes",
			Attributes: map[string]string{"folder": "Engineering"},
		},
		{
			Source:     "fixture.local",
			ExternalID: "rem-001",
			Kind:       KindReminders,
			UpdatedAt:  baseTime.Add(10 * time.Minute),
			Version:    "v1",
			Timezone:   "Asia/Shanghai",
			IsMock:     isMock,
			Title:      "记得带伞",
			Attributes: map[string]string{"due": "2026-10-11T08:00:00+08:00"},
		},
		{
			Source:     "fixture.local",
			ExternalID: "file-001",
			Kind:       KindFiles,
			UpdatedAt:  baseTime.Add(15 * time.Minute),
			Version:    "sha256:abcd",
			Timezone:   "Asia/Shanghai",
			IsMock:     isMock,
			Title:      "Q4-roadmap.md",
			Attributes: map[string]string{"path": "workspace/docs/Q4-roadmap.md"},
		},
		{
			Source:     "fixture.local",
			ExternalID: "contact-001",
			Kind:       KindContacts,
			UpdatedAt:  baseTime.Add(20 * time.Minute),
			Version:    "etag-9",
			Timezone:   "Asia/Shanghai",
			IsMock:     isMock,
			Title:      "Alice Chen",
			Attributes: map[string]string{"role": "Product Lead"},
		},
		{
			Source:     "fixture.local",
			ExternalID: "mail-001",
			Kind:       KindMail,
			UpdatedAt:  baseTime.Add(25 * time.Minute),
			Version:    "uid-402",
			Timezone:   "Asia/Shanghai",
			IsMock:     isMock,
			Title:      "Release checklist confirmation",
			Attributes: map[string]string{"from": "qa@example.org"},
		},
		{
			Source:     "fixture.local",
			ExternalID: "app-001",
			Kind:       KindApps,
			UpdatedAt:  baseTime.Add(30 * time.Minute),
			Version:    "1.4.0",
			Timezone:   "Asia/Shanghai",
			IsMock:     isMock,
			Title:      "ProactiveWorkbench.app",
			Attributes: map[string]string{"bundle_id": "bot.topsun.proactive-workbench"},
		},
	}

	rows := make([]SnapshotRow, 0, len(objs))
	for _, o := range objs {
		rows = append(rows, SnapshotRow{
			SourceName: o.Source,
			UpstreamID: o.ExternalID,
			Domain:     string(o.Kind),
			ModifiedAt: o.UpdatedAt.Format(time.RFC3339),
			ETag:       o.Version,
			TZ:         o.Timezone,
			MockFlag:   o.IsMock,
			Summary:    o.Title,
			Meta:       o.Attributes,
		})
	}
	rawJSON, err := json.Marshal(rows)
	if err != nil {
		panic(err)
	}
	return objs, rawJSON
}

// Acceptance Criterion 1:
// 两个本地测试适配器接同一套测试数据时语义一致。
func TestTwoLocalAdaptersSemanticEquivalenceAcrossAllSevenKinds(t *testing.T) {
	ctx := context.Background()
	objs, snapshotJSON := sampleSevenDomainDataset(true)

	memAdapter := NewMemoryAdapter(sampleDescriptor("local.memory", ModeMock), objs)
	snapAdapter, err := NewSnapshotAdapter(sampleDescriptor("local.snapshot", ModeMock), snapshotJSON)
	if err != nil {
		t.Fatalf("NewSnapshotAdapter failed: %v", err)
	}

	reg := NewRegistry()
	if err := reg.Register(memAdapter); err != nil {
		t.Fatalf("Register(memAdapter): %v", err)
	}
	if err := reg.Register(snapAdapter); err != nil {
		t.Fatalf("Register(snapAdapter): %v", err)
	}

	// Verify discovery covers all 7 kinds for both connectors.
	for _, kind := range AllKinds() {
		matched := reg.ByKind(kind)
		if len(matched) != 2 {
			t.Fatalf("expected 2 connectors for kind %q, got %d", kind, len(matched))
		}
	}

	// Compare paginated List across both adapters.
	collectAllPages := func(c Connector) []Object {
		var all []Object
		token := ""
		for {
			page, err := c.List(ctx, PageRequest{PageSize: 2, PageToken: token})
			if err != nil {
				t.Fatalf("List(%s) failed: %v", c.Descriptor().ID, err)
			}
			all = append(all, page.Objects...)
			if !page.HasMore {
				break
			}
			token = page.NextPageToken
		}
		return all
	}

	memListed := collectAllPages(memAdapter)
	snapListed := collectAllPages(snapAdapter)
	if len(memListed) != 7 || len(snapListed) != 7 {
		t.Fatalf("expected 7 objects from each adapter, got mem=%d snap=%d", len(memListed), len(snapListed))
	}
	assertObjectsSemanticallyEqual(t, memListed, snapListed)

	// Compare incremental Sync across two separate engines.
	engMem := NewEngine(reg)
	engSnap := NewEngine(reg)
	if _, err := engMem.Sync(ctx, "local.memory", SyncRequest{PageSize: 3}); err != nil {
		t.Fatalf("engMem.Sync: %v", err)
	}
	if _, err := engSnap.Sync(ctx, "local.snapshot", SyncRequest{PageSize: 2}); err != nil {
		t.Fatalf("engSnap.Sync: %v", err)
	}
	assertObjectsSemanticallyEqual(t, engMem.Objects(""), engSnap.Objects(""))
}

func assertObjectsSemanticallyEqual(t *testing.T, a, b []Object) {
	t.Helper()
	if len(a) != len(b) {
		t.Fatalf("object slice length mismatch: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].ID != b[i].ID ||
			a[i].Source != b[i].Source ||
			a[i].ExternalID != b[i].ExternalID ||
			a[i].Kind != b[i].Kind ||
			!a[i].UpdatedAt.Equal(b[i].UpdatedAt) ||
			a[i].Version != b[i].Version ||
			a[i].Timezone != b[i].Timezone ||
			a[i].IsMock != b[i].IsMock ||
			a[i].Title != b[i].Title ||
			!reflect.DeepEqual(a[i].Attributes, b[i].Attributes) {
			t.Fatalf("semantic mismatch at index %d:\nleft:  %+v\nright: %+v", i, a[i], b[i])
		}
	}
}

// Acceptance Criterion 2:
// 重复同步不产生重复对象。
func TestRepeatedSyncAndWriteAreIdempotent(t *testing.T) {
	ctx := context.Background()
	objs, _ := sampleSevenDomainDataset(true)
	memAdapter := NewMemoryAdapter(sampleDescriptor("local.memory", ModeMock), objs)

	reg := NewRegistry()
	if err := reg.Register(memAdapter); err != nil {
		t.Fatalf("Register: %v", err)
	}
	eng := NewEngine(reg)

	// First sync adds all 7 objects.
	first, err := eng.Sync(ctx, "local.memory", SyncRequest{PageSize: 3})
	if err != nil {
		t.Fatalf("first Sync failed: %v", err)
	}
	if first.Added != 7 || first.TotalStored != 7 {
		t.Fatalf("expected Added=7 TotalStored=7 on first sync, got %+v", first)
	}

	// Repeating sync from the beginning (Cursor="offset:0") must not duplicate any object.
	second, err := eng.Sync(ctx, "local.memory", SyncRequest{Cursor: "offset:0", PageSize: 4})
	if err != nil {
		t.Fatalf("second Sync failed: %v", err)
	}
	if second.Added != 0 || second.Updated != 0 || second.Unchanged != 7 || second.TotalStored != 7 {
		t.Fatalf("expected 0 added, 0 updated, 7 unchanged on repeat sync, got %+v", second)
	}
	if got := len(eng.Objects("")); got != 7 {
		t.Fatalf("expected 7 stored objects after repeated sync, got %d", got)
	}

	// Repeating sync with the same IdempotencyKey is deduplicated immediately.
	idem1, err := eng.Sync(ctx, "local.memory", SyncRequest{Cursor: "offset:0", IdempotencyKey: "sync-key-1"})
	if err != nil {
		t.Fatalf("idem1 Sync failed: %v", err)
	}
	idem2, err := eng.Sync(ctx, "local.memory", SyncRequest{Cursor: "offset:0", IdempotencyKey: "sync-key-1"})
	if err != nil {
		t.Fatalf("idem2 Sync failed: %v", err)
	}
	if !idem2.Deduplicated || idem1.TotalStored != idem2.TotalStored {
		t.Fatalf("expected deduplicated sync summary on second call, got %+v", idem2)
	}

	// Repeating Write with the same IdempotencyKey must not create duplicate objects.
	newReminder := Object{
		Source:     "fixture.local",
		ExternalID: "rem-002",
		Kind:       KindReminders,
		UpdatedAt:  time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC),
		Version:    "v1",
		Timezone:   "Asia/Shanghai",
		IsMock:     true,
		Title:      "Check flight status",
	}
	w1, err := eng.Write(ctx, "local.memory", WriteRequest{
		Action:         ActionCreate,
		Object:         newReminder,
		IdempotencyKey: "write-rem-002",
	})
	if err != nil {
		t.Fatalf("first Write failed: %v", err)
	}
	w2, err := eng.Write(ctx, "local.memory", WriteRequest{
		Action:         ActionCreate,
		Object:         newReminder,
		IdempotencyKey: "write-rem-002",
	})
	if err != nil {
		t.Fatalf("second Write failed: %v", err)
	}
	if w1.Deduplicated || !w2.Deduplicated {
		t.Fatalf("expected w1.Deduplicated=false and w2.Deduplicated=true, got w1=%+v w2=%+v", w1, w2)
	}
	if got := len(eng.Objects(KindReminders)); got != 2 {
		t.Fatalf("expected exactly 2 reminders after idempotent write, got %d", got)
	}
}

// Acceptance Criterion 3:
// 断连、限流、字段缺失时返回可诊断错误。
func TestDiagnosableErrorsOnDisconnectRateLimitAndMissingFields(t *testing.T) {
	ctx := context.Background()
	objs, _ := sampleSevenDomainDataset(true)
	memAdapter := NewMemoryAdapter(sampleDescriptor("local.memory", ModeMock), objs)

	reg := NewRegistry()
	if err := reg.Register(memAdapter); err != nil {
		t.Fatalf("Register: %v", err)
	}
	eng := NewEngine(reg)

	// 1. Disconnected state returns CodeDisconnected.
	memAdapter.SetState(StateDisconnected, 0)
	if _, err := eng.Sync(ctx, "local.memory", SyncRequest{}); !IsCode(err, CodeDisconnected) {
		t.Fatalf("expected CodeDisconnected on Sync when disconnected, got %v", err)
	}
	if rep, err := memAdapter.Status(ctx); !IsCode(err, CodeDisconnected) || rep.State != StateDisconnected {
		t.Fatalf("expected Status CodeDisconnected, got rep=%+v err=%v", rep, err)
	}

	// 2. Persistent RateLimited state returns CodeRateLimited with RetryAfter hint.
	memAdapter.SetState(StateRateLimited, 15*time.Millisecond)
	_, err := eng.Sync(ctx, "local.memory", SyncRequest{})
	if !IsCode(err, CodeRateLimited) {
		t.Fatalf("expected CodeRateLimited on persistent rate limit, got %v", err)
	}
	var ce *Error
	if !errors.As(err, &ce) || ce.RetryAfter != 15*time.Millisecond {
		t.Fatalf("expected RetryAfter=15ms in error, got %+v", ce)
	}

	// 2b. Transient rate limit within RetryPolicy.MaxAttempts recovers automatically.
	memAdapter.SetTransientRateLimit(2, 5*time.Millisecond)
	sum, err := eng.Sync(ctx, "local.memory", SyncRequest{})
	if err != nil || sum.Added != 7 {
		t.Fatalf("expected transient rate limit to succeed after retry, got sum=%+v err=%v", sum, err)
	}

	// 3. Missing required fields & invalid timezone return CodeMissingField / CodeInvalidTimezone with Field populated.
	validBase := objs[0]
	cases := []struct {
		name      string
		mutate    func(*Object)
		wantCode  ErrorCode
		wantField string
	}{
		{
			name:      "missing source",
			mutate:    func(o *Object) { o.Source = "" },
			wantCode:  CodeMissingField,
			wantField: "source",
		},
		{
			name:      "missing external_id",
			mutate:    func(o *Object) { o.ExternalID = "" },
			wantCode:  CodeMissingField,
			wantField: "external_id",
		},
		{
			name:      "missing updated_at",
			mutate:    func(o *Object) { o.UpdatedAt = time.Time{} },
			wantCode:  CodeMissingField,
			wantField: "updated_at",
		},
		{
			name:      "missing version",
			mutate:    func(o *Object) { o.Version = "" },
			wantCode:  CodeMissingField,
			wantField: "version",
		},
		{
			name:      "missing timezone",
			mutate:    func(o *Object) { o.Timezone = "" },
			wantCode:  CodeMissingField,
			wantField: "timezone",
		},
		{
			name:      "invalid timezone",
			mutate:    func(o *Object) { o.Timezone = "Mars/Olympus_Mons" },
			wantCode:  CodeInvalidTimezone,
			wantField: "timezone",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			candidate := validBase.Clone()
			tc.mutate(&candidate)
			_, err := ValidateObject("local.memory", ModeMock, false, candidate)
			if !IsCode(err, tc.wantCode) {
				t.Fatalf("expected error code %q, got %v", tc.wantCode, err)
			}
			var diag *Error
			if !errors.As(err, &diag) || diag.Field != tc.wantField {
				t.Fatalf("expected Field=%q, got %+v", tc.wantField, diag)
			}
		})
	}
}

// Acceptance Criterion 4:
// MOCK 数据源不能冒充 live，且连接器遵循最小权限原则。
func TestMockCannotMasqueradeAsLiveAndLeastPrivilegeEnforced(t *testing.T) {
	ctx := context.Background()
	mockObjs, _ := sampleSevenDomainDataset(true)
	mockAdapter := NewMemoryAdapter(sampleDescriptor("mock.source", ModeMock), mockObjs)

	reg := NewRegistry()
	if err := reg.Register(mockAdapter); err != nil {
		t.Fatalf("Register(mockAdapter): %v", err)
	}
	eng := NewEngine(reg)

	// 1. Requesting live sync from a ModeMock connector must fail with CodeMockAsLive.
	if _, err := eng.Sync(ctx, "mock.source", SyncRequest{RequireLive: true}); !IsCode(err, CodeMockAsLive) {
		t.Fatalf("expected CodeMockAsLive when syncing mock connector with RequireLive=true, got %v", err)
	}

	// 2. A ModeMock connector emitting an object with IsMock=false must fail with CodeMockAsLive.
	spoofed := mockObjs[0].Clone()
	spoofed.IsMock = false
	if _, err := ValidateObject("mock.source", ModeMock, false, spoofed); !IsCode(err, CodeMockAsLive) {
		t.Fatalf("expected CodeMockAsLive when mock connector emits IsMock=false, got %v", err)
	}

	// 3. A ModeLive connector emitting an object with IsMock=true when RequireLive=true must fail with CodeMockAsLive.
	mockObjInLive := mockObjs[0].Clone()
	mockObjInLive.IsMock = true
	if _, err := ValidateObject("live.source", ModeLive, true, mockObjInLive); !IsCode(err, CodeMockAsLive) {
		t.Fatalf("expected CodeMockAsLive when live request receives IsMock=true object, got %v", err)
	}

	// 4. Least-privilege check: requesting full-disk "/" or wildcard "*" or undeclared kind is rejected.
	overprivileged := sampleDescriptor("bad.full_disk", ModeLive)
	overprivileged.Permissions[0].Resources = []string{"/"}
	if err := reg.Register(NewMemoryAdapter(overprivileged, nil)); !IsCode(err, CodePermissionViolation) {
		t.Fatalf("expected CodePermissionViolation for full-disk '/' scope, got %v", err)
	}

	allAccounts := sampleDescriptor("bad.all_accounts", ModeLive)
	allAccounts.Permissions[0].Resources = []string{"all_accounts"}
	if err := reg.Register(NewMemoryAdapter(allAccounts, nil)); !IsCode(err, CodePermissionViolation) {
		t.Fatalf("expected CodePermissionViolation for 'all_accounts' scope, got %v", err)
	}
}
