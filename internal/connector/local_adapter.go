package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// MemoryAdapter is a local in-memory Connector implementation for contract testing
// and fixture-driven development.
type MemoryAdapter struct {
	mu               sync.Mutex
	desc             Descriptor
	state            ConnectionState
	retryAfter       time.Duration
	rateLimitBudget  int // Number of rate-limited failures before succeeding.
	records          []Object
	idempotencyCache map[string]WriteResult
}

// NewMemoryAdapter constructs a MemoryAdapter with the given descriptor and initial objects.
func NewMemoryAdapter(desc Descriptor, initial []Object) *MemoryAdapter {
	cloned := make([]Object, 0, len(initial))
	for _, o := range initial {
		cloned = append(cloned, o.Clone())
	}
	return &MemoryAdapter{
		desc:             desc,
		state:            StateConnected,
		records:          cloned,
		idempotencyCache: make(map[string]WriteResult),
	}
}

// SetState configures the adapter's connection state (e.g. StateDisconnected, StateRateLimited).
func (m *MemoryAdapter) SetState(state ConnectionState, retryAfter time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = state
	m.retryAfter = retryAfter
}

// SetTransientRateLimit configures the adapter to fail with CodeRateLimited for the next
// failCount calls before recovering to StateConnected.
func (m *MemoryAdapter) SetTransientRateLimit(failCount int, retryAfter time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = StateRateLimited
	m.rateLimitBudget = failCount
	m.retryAfter = retryAfter
}

func (m *MemoryAdapter) checkStateLocked() error {
	switch m.state {
	case StateDisconnected:
		return Disconnected(m.desc.ID, "memory adapter is disconnected")
	case StateRateLimited:
		if m.rateLimitBudget > 0 {
			m.rateLimitBudget--
			if m.rateLimitBudget == 0 {
				m.state = StateConnected
			}
		}
		return RateLimited(m.desc.ID, m.retryAfter, "memory adapter rate limit exceeded")
	case StateUnauthorized:
		return &Error{
			Code:        CodeUnauthorized,
			ConnectorID: m.desc.ID,
			Message:     "memory adapter is unauthorized",
		}
	default:
		return nil
	}
}

// Descriptor returns the connector's capability manifest.
func (m *MemoryAdapter) Descriptor() Descriptor {
	return m.desc
}

// Status reports the current connection state of the adapter.
func (m *MemoryAdapter) Status(_ context.Context) (StatusReport, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rep := StatusReport{
		ConnectorID: m.desc.ID,
		State:       m.state,
		Mode:        m.desc.Mode,
		CheckedAt:   time.Now().UTC(),
		RetryAfter:  m.retryAfter,
	}
	if err := m.checkStateLocked(); err != nil {
		rep.Detail = err.Error()
		return rep, err
	}
	rep.Detail = "ok"
	return rep, nil
}

// List returns a deterministic page of canonical objects matching req.Kind.
func (m *MemoryAdapter) List(_ context.Context, req PageRequest) (PageResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.checkStateLocked(); err != nil {
		return PageResult{}, err
	}
	if req.RequireLive && m.desc.Mode == ModeMock {
		return PageResult{}, MockAsLive(m.desc.ID, "memory adapter is in mock mode")
	}

	filtered := m.filteredAndSortedLocked(req.Kind)
	validated := make([]Object, 0, len(filtered))
	for _, raw := range filtered {
		norm, err := ValidateObject(m.desc.ID, m.desc.Mode, req.RequireLive, raw)
		if err != nil {
			return PageResult{}, err
		}
		validated = append(validated, norm)
	}
	return paginateObjects(validated, req.PageSize, req.PageToken)
}

// Sync returns incremental updates starting at req.Cursor.
func (m *MemoryAdapter) Sync(_ context.Context, req SyncRequest) (SyncResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.checkStateLocked(); err != nil {
		return SyncResult{}, err
	}
	if req.RequireLive && m.desc.Mode == ModeMock {
		return SyncResult{}, MockAsLive(m.desc.ID, "memory adapter is in mock mode")
	}

	filtered := m.filteredAndSortedLocked(req.Kind)
	validated := make([]Object, 0, len(filtered))
	for _, raw := range filtered {
		norm, err := ValidateObject(m.desc.ID, m.desc.Mode, req.RequireLive, raw)
		if err != nil {
			return SyncResult{}, err
		}
		validated = append(validated, norm)
	}
	return syncSlice(validated, req)
}

// Write creates, updates, or deletes an object in the memory adapter.
func (m *MemoryAdapter) Write(_ context.Context, req WriteRequest) (WriteResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.checkStateLocked(); err != nil {
		return WriteResult{}, err
	}
	if req.IdempotencyKey != "" {
		if prev, ok := m.idempotencyCache[req.IdempotencyKey]; ok {
			prev.Deduplicated = true
			return prev, nil
		}
	}
	norm, err := ValidateObject(m.desc.ID, m.desc.Mode, req.RequireLive, req.Object)
	if err != nil {
		return WriteResult{}, err
	}

	switch req.Action {
	case ActionCreate, ActionUpdate:
		replaced := false
		for i := range m.records {
			if m.records[i].Kind == norm.Kind && m.records[i].ExternalID == norm.ExternalID {
				m.records[i] = norm
				replaced = true
				break
			}
		}
		if !replaced {
			m.records = append(m.records, norm)
		}
	case ActionDelete:
		norm.Deleted = true
		for i := range m.records {
			if m.records[i].Kind == norm.Kind && m.records[i].ExternalID == norm.ExternalID {
				m.records[i] = norm
				break
			}
		}
	default:
		return WriteResult{}, &Error{
			Code:        CodeUnsupportedAction,
			ConnectorID: m.desc.ID,
			Message:     fmt.Sprintf("unsupported write action %q", req.Action),
		}
	}

	res := WriteResult{
		Object:         norm,
		IdempotencyKey: req.IdempotencyKey,
	}
	if req.IdempotencyKey != "" {
		m.idempotencyCache[req.IdempotencyKey] = res
	}
	return res, nil
}

func (m *MemoryAdapter) filteredAndSortedLocked(kind Kind) []Object {
	var out []Object
	for _, r := range m.records {
		if kind != "" && r.Kind != kind {
			continue
		}
		out = append(out, r.Clone())
	}
	sort.Slice(out, func(i, j int) bool {
		return CanonicalID(out[i].Kind, out[i].ExternalID) < CanonicalID(out[j].Kind, out[j].ExternalID)
	})
	return out
}

// SnapshotRow represents a flat serialized record (as produced by local JSON/ICS/VCF
// snapshot exports) ingested by SnapshotAdapter.
type SnapshotRow struct {
	SourceName string            `json:"source_name"`
	UpstreamID string            `json:"upstream_id"`
	Domain     string            `json:"domain"`
	ModifiedAt string            `json:"modified_at"` // RFC3339 timestamp
	ETag       string            `json:"etag"`
	TZ         string            `json:"tz"`
	MockFlag   bool              `json:"mock_flag"`
	Summary    string            `json:"summary"`
	Meta       map[string]string `json:"meta"`
	Tombstone  bool              `json:"tombstone"`
}

// SnapshotAdapter is the second local test adapter. It ingests flat JSON snapshot rows
// and maps them to canonical connector Objects, verifying that different adapter
// representations produce identical canonical semantics.
type SnapshotAdapter struct {
	mu               sync.Mutex
	desc             Descriptor
	state            ConnectionState
	retryAfter       time.Duration
	rows             []SnapshotRow
	idempotencyCache map[string]WriteResult
}

// NewSnapshotAdapter constructs a SnapshotAdapter from JSON-encoded snapshot rows.
func NewSnapshotAdapter(desc Descriptor, jsonPayload []byte) (*SnapshotAdapter, error) {
	var rows []SnapshotRow
	if len(strings.TrimSpace(string(jsonPayload))) > 0 {
		if err := json.Unmarshal(jsonPayload, &rows); err != nil {
			return nil, &Error{
				Code:        CodeMissingField,
				ConnectorID: desc.ID,
				Field:       "snapshot_json",
				Message:     "invalid snapshot JSON payload",
				Cause:       err,
			}
		}
	}
	return &SnapshotAdapter{
		desc:             desc,
		state:            StateConnected,
		rows:             rows,
		idempotencyCache: make(map[string]WriteResult),
	}, nil
}

// SetState configures the snapshot adapter's connection state.
func (s *SnapshotAdapter) SetState(state ConnectionState, retryAfter time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = state
	s.retryAfter = retryAfter
}

func (s *SnapshotAdapter) checkStateLocked() error {
	switch s.state {
	case StateDisconnected:
		return Disconnected(s.desc.ID, "snapshot adapter is disconnected")
	case StateRateLimited:
		return RateLimited(s.desc.ID, s.retryAfter, "snapshot adapter rate limit exceeded")
	case StateUnauthorized:
		return &Error{
			Code:        CodeUnauthorized,
			ConnectorID: s.desc.ID,
			Message:     "snapshot adapter is unauthorized",
		}
	default:
		return nil
	}
}

// Descriptor returns the connector's capability manifest.
func (s *SnapshotAdapter) Descriptor() Descriptor {
	return s.desc
}

// Status reports the current connection state of the snapshot adapter.
func (s *SnapshotAdapter) Status(_ context.Context) (StatusReport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rep := StatusReport{
		ConnectorID: s.desc.ID,
		State:       s.state,
		Mode:        s.desc.Mode,
		CheckedAt:   time.Now().UTC(),
		RetryAfter:  s.retryAfter,
	}
	if err := s.checkStateLocked(); err != nil {
		rep.Detail = err.Error()
		return rep, err
	}
	rep.Detail = "ok"
	return rep, nil
}

func (s *SnapshotAdapter) mapRowToObject(r SnapshotRow, requireLive bool) (Object, error) {
	var ts time.Time
	if strings.TrimSpace(r.ModifiedAt) != "" {
		parsed, err := time.Parse(time.RFC3339, r.ModifiedAt)
		if err != nil {
			return Object{}, MissingField(s.desc.ID, "updated_at", fmt.Sprintf("invalid RFC3339 modified_at %q", r.ModifiedAt))
		}
		ts = parsed
	}
	raw := Object{
		Source:     r.SourceName,
		ExternalID: r.UpstreamID,
		Kind:       Kind(strings.TrimSpace(r.Domain)),
		UpdatedAt:  ts,
		Version:    r.ETag,
		Timezone:   r.TZ,
		IsMock:     r.MockFlag,
		Title:      r.Summary,
		Attributes: r.Meta,
		Deleted:    r.Tombstone,
	}
	return ValidateObject(s.desc.ID, s.desc.Mode, requireLive, raw)
}

func (s *SnapshotAdapter) collectValidatedLocked(kind Kind, requireLive bool) ([]Object, error) {
	var out []Object
	for _, r := range s.rows {
		if kind != "" && Kind(strings.TrimSpace(r.Domain)) != kind {
			continue
		}
		obj, err := s.mapRowToObject(r, requireLive)
		if err != nil {
			return nil, err
		}
		out = append(out, obj)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// List returns a paginated slice of canonical objects mapped from the snapshot rows.
func (s *SnapshotAdapter) List(_ context.Context, req PageRequest) (PageResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkStateLocked(); err != nil {
		return PageResult{}, err
	}
	if req.RequireLive && s.desc.Mode == ModeMock {
		return PageResult{}, MockAsLive(s.desc.ID, "snapshot adapter is in mock mode")
	}
	objs, err := s.collectValidatedLocked(req.Kind, req.RequireLive)
	if err != nil {
		return PageResult{}, err
	}
	return paginateObjects(objs, req.PageSize, req.PageToken)
}

// Sync performs cursor-based incremental sync over the snapshot rows.
func (s *SnapshotAdapter) Sync(_ context.Context, req SyncRequest) (SyncResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkStateLocked(); err != nil {
		return SyncResult{}, err
	}
	if req.RequireLive && s.desc.Mode == ModeMock {
		return SyncResult{}, MockAsLive(s.desc.ID, "snapshot adapter is in mock mode")
	}
	objs, err := s.collectValidatedLocked(req.Kind, req.RequireLive)
	if err != nil {
		return SyncResult{}, err
	}
	return syncSlice(objs, req)
}

// Write creates, updates, or deletes a snapshot row with idempotency support.
func (s *SnapshotAdapter) Write(_ context.Context, req WriteRequest) (WriteResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkStateLocked(); err != nil {
		return WriteResult{}, err
	}
	if req.IdempotencyKey != "" {
		if prev, ok := s.idempotencyCache[req.IdempotencyKey]; ok {
			prev.Deduplicated = true
			return prev, nil
		}
	}
	norm, err := ValidateObject(s.desc.ID, s.desc.Mode, req.RequireLive, req.Object)
	if err != nil {
		return WriteResult{}, err
	}

	row := SnapshotRow{
		SourceName: norm.Source,
		UpstreamID: norm.ExternalID,
		Domain:     string(norm.Kind),
		ModifiedAt: norm.UpdatedAt.Format(time.RFC3339),
		ETag:       norm.Version,
		TZ:         norm.Timezone,
		MockFlag:   norm.IsMock,
		Summary:    norm.Title,
		Meta:       norm.Attributes,
		Tombstone:  req.Action == ActionDelete || norm.Deleted,
	}
	norm.Deleted = row.Tombstone

	switch req.Action {
	case ActionCreate, ActionUpdate, ActionDelete:
		replaced := false
		for i := range s.rows {
			if Kind(s.rows[i].Domain) == norm.Kind && s.rows[i].UpstreamID == norm.ExternalID {
				s.rows[i] = row
				replaced = true
				break
			}
		}
		if !replaced {
			s.rows = append(s.rows, row)
		}
	default:
		return WriteResult{}, &Error{
			Code:        CodeUnsupportedAction,
			ConnectorID: s.desc.ID,
			Message:     fmt.Sprintf("unsupported write action %q", req.Action),
		}
	}

	res := WriteResult{
		Object:         norm,
		IdempotencyKey: req.IdempotencyKey,
	}
	if req.IdempotencyKey != "" {
		s.idempotencyCache[req.IdempotencyKey] = res
	}
	return res, nil
}

func paginateObjects(objs []Object, pageSize int, pageToken string) (PageResult, error) {
	start := 0
	if pageToken != "" {
		idx, err := strconv.Atoi(pageToken)
		if err != nil || idx < 0 || idx > len(objs) {
			return PageResult{}, MissingField("", "page_token", fmt.Sprintf("invalid page_token %q", pageToken))
		}
		start = idx
	}
	if pageSize <= 0 {
		pageSize = len(objs) - start
		if pageSize <= 0 {
			pageSize = 1
		}
	}
	end := start + pageSize
	if end > len(objs) {
		end = len(objs)
	}
	slice := make([]Object, 0, end-start)
	for _, o := range objs[start:end] {
		slice = append(slice, o.Clone())
	}
	hasMore := end < len(objs)
	nextToken := ""
	if hasMore {
		nextToken = strconv.Itoa(end)
	}
	return PageResult{
		Objects:       slice,
		NextPageToken: nextToken,
		HasMore:       hasMore,
	}, nil
}

func syncSlice(objs []Object, req SyncRequest) (SyncResult, error) {
	start := 0
	if req.Cursor != "" {
		trimmed := strings.TrimPrefix(string(req.Cursor), "offset:")
		idx, err := strconv.Atoi(trimmed)
		if err != nil || idx < 0 || idx > len(objs) {
			return SyncResult{}, MissingField("", "cursor", fmt.Sprintf("invalid sync cursor %q", req.Cursor))
		}
		start = idx
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = len(objs) - start
		if pageSize <= 0 {
			pageSize = 1
		}
	}
	end := start + pageSize
	if end > len(objs) {
		end = len(objs)
	}
	batch := make([]Object, 0, end-start)
	for _, o := range objs[start:end] {
		batch = append(batch, o.Clone())
	}
	hasMore := end < len(objs)
	nextCursor := SyncCursor(fmt.Sprintf("offset:%d", end))
	return SyncResult{
		Objects:        batch,
		NextCursor:     nextCursor,
		HasMore:        hasMore,
		IdempotencyKey: req.IdempotencyKey,
		SyncedAt:       time.Now().UTC(),
	}, nil
}
