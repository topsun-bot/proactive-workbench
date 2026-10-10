package connector

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"
)

// SyncSummary reports the deduplicated outcome of running an incremental sync
// through Engine.
type SyncSummary struct {
	ConnectorID    string
	Kind           Kind
	Added          int
	Updated        int
	Unchanged      int
	Deleted        int
	TotalStored    int
	NextCursor     SyncCursor
	IdempotencyKey string
	Deduplicated   bool
}

// Engine coordinates discovery, validation, retry, and idempotent incremental sync
// across registered connectors.
type Engine struct {
	mu          sync.RWMutex
	registry    *Registry
	objects     map[string]Object
	cursors     map[string]SyncCursor
	syncLedger  map[string]SyncSummary
	writeLedger map[string]WriteResult
	sleepFn     func(time.Duration)
}

// NewEngine creates a sync engine backed by reg.
func NewEngine(reg *Registry) *Engine {
	if reg == nil {
		reg = NewRegistry()
	}
	return &Engine{
		registry:    reg,
		objects:     make(map[string]Object),
		cursors:     make(map[string]SyncCursor),
		syncLedger:  make(map[string]SyncSummary),
		writeLedger: make(map[string]WriteResult),
		sleepFn:     func(time.Duration) {},
	}
}

func cursorKey(connectorID string, kind Kind) string {
	return connectorID + "::" + string(kind)
}

// Sync runs incremental sync for connectorID and req.Kind, validating every returned
// object, deduplicating by canonical ID + version, and advancing the stored cursor.
func (e *Engine) Sync(ctx context.Context, connectorID string, req SyncRequest) (SyncSummary, error) {
	c, ok := e.registry.Get(connectorID)
	if !ok {
		return SyncSummary{}, &Error{
			Code:        CodeUnknownConnector,
			ConnectorID: connectorID,
			Message:     "connector is not registered",
		}
	}
	desc := c.Descriptor()
	if req.RequireLive && desc.Mode == ModeMock {
		return SyncSummary{}, MockAsLive(connectorID, "mock connector rejected because RequireLive=true")
	}

	if req.IdempotencyKey != "" {
		e.mu.RLock()
		prev, exists := e.syncLedger[connectorID+"::"+req.IdempotencyKey]
		e.mu.RUnlock()
		if exists {
			prev.Deduplicated = true
			return prev, nil
		}
	}

	e.mu.RLock()
	if req.Cursor == "" {
		req.Cursor = e.cursors[cursorKey(connectorID, req.Kind)]
	}
	e.mu.RUnlock()

	var summary SyncSummary
	summary.ConnectorID = connectorID
	summary.Kind = req.Kind
	summary.IdempotencyKey = req.IdempotencyKey

	for {
		res, err := e.callSyncWithRetry(ctx, c, req)
		if err != nil {
			return SyncSummary{}, err
		}

		validated := make([]Object, 0, len(res.Objects))
		for _, raw := range res.Objects {
			norm, verr := ValidateObject(connectorID, desc.Mode, req.RequireLive, raw)
			if verr != nil {
				return SyncSummary{}, verr
			}
			validated = append(validated, norm)
		}

		e.mu.Lock()
		for _, obj := range validated {
			existing, found := e.objects[obj.ID]
			if obj.Deleted {
				if found {
					delete(e.objects, obj.ID)
					summary.Deleted++
				} else {
					summary.Unchanged++
				}
				continue
			}
			if !found {
				e.objects[obj.ID] = obj
				summary.Added++
				continue
			}
			if existing.Version == obj.Version && existing.UpdatedAt.Equal(obj.UpdatedAt) {
				summary.Unchanged++
				continue
			}
			e.objects[obj.ID] = obj
			summary.Updated++
		}
		if res.NextCursor != "" {
			e.cursors[cursorKey(connectorID, req.Kind)] = res.NextCursor
			summary.NextCursor = res.NextCursor
		}
		summary.TotalStored = len(e.objects)
		e.mu.Unlock()

		if !res.HasMore || res.NextCursor == "" || res.NextCursor == req.Cursor {
			break
		}
		req.Cursor = res.NextCursor
	}

	if req.IdempotencyKey != "" {
		e.mu.Lock()
		e.syncLedger[connectorID+"::"+req.IdempotencyKey] = summary
		e.mu.Unlock()
	}
	return summary, nil
}

func (e *Engine) callSyncWithRetry(ctx context.Context, c Connector, req SyncRequest) (SyncResult, error) {
	desc := c.Descriptor()
	maxAttempts := desc.RetryPolicy.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		res, err := c.Sync(ctx, req)
		if err == nil {
			return res, nil
		}
		lastErr = err
		var ce *Error
		if !errors.As(err, &ce) || ce.Code != CodeRateLimited || attempt == maxAttempts {
			return SyncResult{}, err
		}
		wait := ce.RetryAfter
		if wait <= 0 {
			wait = desc.RetryPolicy.InitialBackoff
		}
		if desc.RetryPolicy.MaxBackoff > 0 && wait > desc.RetryPolicy.MaxBackoff {
			wait = desc.RetryPolicy.MaxBackoff
		}
		if e.sleepFn != nil && wait > 0 {
			e.sleepFn(wait)
		}
	}
	return SyncResult{}, lastErr
}

// Write executes an idempotent write through the target connector and updates the engine store.
func (e *Engine) Write(ctx context.Context, connectorID string, req WriteRequest) (WriteResult, error) {
	c, ok := e.registry.Get(connectorID)
	if !ok {
		return WriteResult{}, &Error{
			Code:        CodeUnknownConnector,
			ConnectorID: connectorID,
			Message:     "connector is not registered",
		}
	}
	desc := c.Descriptor()
	if req.RequireLive && desc.Mode == ModeMock {
		return WriteResult{}, MockAsLive(connectorID, "mock connector rejected because RequireLive=true")
	}

	if req.IdempotencyKey != "" {
		e.mu.RLock()
		prev, exists := e.writeLedger[connectorID+"::"+req.IdempotencyKey]
		e.mu.RUnlock()
		if exists {
			prev.Deduplicated = true
			return prev, nil
		}
	}

	norm, err := ValidateObject(connectorID, desc.Mode, req.RequireLive, req.Object)
	if err != nil {
		return WriteResult{}, err
	}
	req.Object = norm

	res, err := c.Write(ctx, req)
	if err != nil {
		return WriteResult{}, err
	}
	normOut, err := ValidateObject(connectorID, desc.Mode, req.RequireLive, res.Object)
	if err != nil {
		return WriteResult{}, err
	}
	res.Object = normOut

	e.mu.Lock()
	if req.Action == ActionDelete || normOut.Deleted {
		delete(e.objects, normOut.ID)
	} else {
		e.objects[normOut.ID] = normOut
	}
	if req.IdempotencyKey != "" {
		e.writeLedger[connectorID+"::"+req.IdempotencyKey] = res
	}
	e.mu.Unlock()

	return res, nil
}

// Objects returns all non-deleted canonical objects for kind (or all kinds if kind == ""),
// sorted deterministically by canonical ID.
func (e *Engine) Objects(kind Kind) []Object {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]Object, 0, len(e.objects))
	for _, obj := range e.objects {
		if kind != "" && obj.Kind != kind {
			continue
		}
		out = append(out, obj.Clone())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
