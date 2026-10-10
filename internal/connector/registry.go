package connector

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// ValidateDescriptor verifies a connector descriptor, including its capability
// declarations and least-privilege permission scopes.
func ValidateDescriptor(d Descriptor) error {
	id := strings.TrimSpace(d.ID)
	if id == "" {
		return MissingField("", "id", "connector descriptor ID is required")
	}
	if !d.Mode.Valid() {
		return &Error{
			Code:        CodeMissingField,
			ConnectorID: id,
			Field:       "mode",
			Message:     fmt.Sprintf("connector mode must be %q or %q, got %q", ModeMock, ModeLive, d.Mode),
		}
	}
	if len(d.SupportedKinds) == 0 {
		return MissingField(id, "supported_kinds", "at least one supported kind is required")
	}
	allowedKinds := make(map[Kind]bool, len(d.SupportedKinds))
	for _, k := range d.SupportedKinds {
		if !k.Valid() {
			return &Error{
				Code:        CodeInvalidKind,
				ConnectorID: id,
				Field:       "supported_kinds",
				Message:     fmt.Sprintf("unsupported connector kind %q", k),
			}
		}
		allowedKinds[k] = true
	}

	if len(d.SupportedActions) == 0 {
		return MissingField(id, "supported_actions", "at least one supported action is required")
	}
	allowedActions := make(map[Action]bool, len(d.SupportedActions))
	for _, a := range d.SupportedActions {
		if !a.Valid() {
			return &Error{
				Code:        CodeUnsupportedAction,
				ConnectorID: id,
				Field:       "supported_actions",
				Message:     fmt.Sprintf("unsupported connector action %q", a),
			}
		}
		allowedActions[a] = true
	}

	if len(d.Permissions) == 0 {
		return &Error{
			Code:        CodePermissionViolation,
			ConnectorID: id,
			Field:       "permissions",
			Message:     "connector must explicitly declare minimal permission scopes",
		}
	}

	for _, p := range d.Permissions {
		if !p.Kind.Valid() || !allowedKinds[p.Kind] {
			return &Error{
				Code:        CodePermissionViolation,
				ConnectorID: id,
				Field:       "permissions.kind",
				Message:     fmt.Sprintf("permission requests undeclared kind %q", p.Kind),
			}
		}
		if len(p.Actions) == 0 {
			return &Error{
				Code:        CodePermissionViolation,
				ConnectorID: id,
				Field:       "permissions.actions",
				Message:     "permission scope must declare at least one action",
			}
		}
		for _, a := range p.Actions {
			if !allowedActions[a] {
				return &Error{
					Code:        CodePermissionViolation,
					ConnectorID: id,
					Field:       "permissions.actions",
					Message:     fmt.Sprintf("permission requests undeclared action %q", a),
				}
			}
		}
		if len(p.Resources) == 0 {
			return &Error{
				Code:        CodePermissionViolation,
				ConnectorID: id,
				Field:       "permissions.resources",
				Message:     "permission scope must list explicit scoped resources",
			}
		}
		for _, res := range p.Resources {
			trimmed := strings.TrimSpace(res)
			lower := strings.ToLower(trimmed)
			if trimmed == "" || trimmed == "*" || trimmed == "/" ||
				lower == "full_disk_access" || lower == "all_accounts" || lower == "all_files" {
				return &Error{
					Code:        CodePermissionViolation,
					ConnectorID: id,
					Field:       "permissions.resources",
					Message:     fmt.Sprintf("least-privilege violation: broad resource %q is forbidden", res),
				}
			}
		}
		if strings.TrimSpace(p.Reason) == "" {
			return &Error{
				Code:        CodePermissionViolation,
				ConnectorID: id,
				Field:       "permissions.reason",
				Message:     "permission scope must state a user-visible reason",
			}
		}
	}
	return nil
}

// ValidateObject enforces the canonical external object schema and prevents
// MOCK connectors or objects from masquerading as live data.
func ValidateObject(connectorID string, connectorMode Mode, requireLive bool, obj Object) (Object, error) {
	if requireLive && connectorMode == ModeMock {
		return Object{}, MockAsLive(connectorID, "mock connector cannot satisfy a live-only request")
	}
	if strings.TrimSpace(obj.Source) == "" {
		return Object{}, MissingField(connectorID, "source", "object source is required")
	}
	if strings.TrimSpace(obj.ExternalID) == "" {
		return Object{}, MissingField(connectorID, "external_id", "object external_id is required")
	}
	if !obj.Kind.Valid() {
		if obj.Kind == "" {
			return Object{}, MissingField(connectorID, "kind", "object kind is required")
		}
		return Object{}, &Error{
			Code:        CodeInvalidKind,
			ConnectorID: connectorID,
			Field:       "kind",
			Message:     fmt.Sprintf("invalid object kind %q", obj.Kind),
		}
	}
	if obj.UpdatedAt.IsZero() {
		return Object{}, MissingField(connectorID, "updated_at", "object updated_at is required")
	}
	if strings.TrimSpace(obj.Version) == "" {
		return Object{}, MissingField(connectorID, "version", "object version is required")
	}
	tz := strings.TrimSpace(obj.Timezone)
	if tz == "" {
		return Object{}, MissingField(connectorID, "timezone", "object timezone is required")
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return Object{}, InvalidTimezone(connectorID, tz, err)
	}
	if connectorMode == ModeMock && !obj.IsMock {
		return Object{}, MockAsLive(connectorID, fmt.Sprintf("object %q from mock connector must set IsMock=true", obj.ExternalID))
	}
	if requireLive && obj.IsMock {
		return Object{}, MockAsLive(connectorID, fmt.Sprintf("mock object %q cannot be accepted as live data", obj.ExternalID))
	}

	normalized := obj.Clone()
	normalized.ID = CanonicalID(normalized.Kind, normalized.ExternalID)
	normalized.Timezone = tz
	normalized.UpdatedAt = normalized.UpdatedAt.In(loc)
	return normalized, nil
}

// Registry provides thread-safe registration and discovery of connectors.
// Adding a new data source requires only implementing Connector and registering it
// here, without modifying the memory store or proactive reasoning core.
type Registry struct {
	mu         sync.RWMutex
	connectors map[string]Connector
}

// NewRegistry creates an empty connector registry.
func NewRegistry() *Registry {
	return &Registry{
		connectors: make(map[string]Connector),
	}
}

// Register validates and registers a connector by its Descriptor().ID.
func (r *Registry) Register(c Connector) error {
	if c == nil {
		return MissingField("", "connector", "connector cannot be nil")
	}
	desc := c.Descriptor()
	if err := ValidateDescriptor(desc); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.connectors[desc.ID] = c
	return nil
}

// Get retrieves a registered connector by ID.
func (r *Registry) Get(id string) (Connector, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.connectors[id]
	return c, ok
}

// Descriptors returns all registered connector descriptors sorted by ID.
func (r *Registry) Descriptors() []Descriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Descriptor, 0, len(r.connectors))
	for _, c := range r.connectors {
		out = append(out, c.Descriptor())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// ByKind returns all connectors that declare support for the given kind, sorted by ID.
func (r *Registry) ByKind(kind Kind) []Connector {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []Connector
	for _, c := range r.connectors {
		for _, k := range c.Descriptor().SupportedKinds {
			if k == kind {
				out = append(out, c)
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Descriptor().ID < out[j].Descriptor().ID })
	return out
}
