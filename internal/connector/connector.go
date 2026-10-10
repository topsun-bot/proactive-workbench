package connector

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Kind identifies one of the seven first-class external data domains supported
// by the workbench connector framework.
type Kind string

const (
	KindCalendar  Kind = "calendar"
	KindNotes     Kind = "notes"
	KindReminders Kind = "reminders"
	KindFiles     Kind = "files"
	KindContacts  Kind = "contacts"
	KindMail      Kind = "mail"
	KindApps      Kind = "apps"
)

// AllKinds returns the seven canonical connector data kinds in deterministic order.
func AllKinds() []Kind {
	return []Kind{
		KindCalendar,
		KindNotes,
		KindReminders,
		KindFiles,
		KindContacts,
		KindMail,
		KindApps,
	}
}

// Valid reports whether k is one of the seven canonical connector kinds.
func (k Kind) Valid() bool {
	switch k {
	case KindCalendar, KindNotes, KindReminders, KindFiles, KindContacts, KindMail, KindApps:
		return true
	default:
		return false
	}
}

// Action identifies a read or write operation supported by a connector.
type Action string

const (
	ActionRead   Action = "read"
	ActionList   Action = "list"
	ActionSync   Action = "sync"
	ActionCreate Action = "create"
	ActionUpdate Action = "update"
	ActionDelete Action = "delete"
)

// Valid reports whether a is a recognized connector action.
func (a Action) Valid() bool {
	switch a {
	case ActionRead, ActionList, ActionSync, ActionCreate, ActionUpdate, ActionDelete:
		return true
	default:
		return false
	}
}

// Mode distinguishes synthetic/fixture connectors from live connectors.
type Mode string

const (
	ModeMock Mode = "mock"
	ModeLive Mode = "live"
)

// Valid reports whether m is ModeMock or ModeLive.
func (m Mode) Valid() bool {
	return m == ModeMock || m == ModeLive
}

// ConnectionState describes the runtime health of a connector.
type ConnectionState string

const (
	StateConnected    ConnectionState = "connected"
	StateDisconnected ConnectionState = "disconnected"
	StateRateLimited  ConnectionState = "rate_limited"
	StateUnauthorized ConnectionState = "unauthorized"
	StateDegraded     ConnectionState = "degraded"
)

// PermissionScope declares a minimal, scoped permission required by a connector.
// Connectors must declare only the specific kinds, actions, and scoped collections/paths
// they require, and never request full-disk or full-account wildcard access.
type PermissionScope struct {
	Kind      Kind
	Actions   []Action
	Resources []string // Scoped calendars, folders, or mailboxes (never "*", "/", or "all_accounts").
	Reason    string
}

// RetryPolicy declares how the sync engine or caller should retry transient errors.
type RetryPolicy struct {
	MaxAttempts    int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
}

// Descriptor declares a connector's identity, mode, supported kinds/actions,
// minimal permission scopes, and retry policy.
type Descriptor struct {
	ID               string
	DisplayName      string
	Summary          string
	Mode             Mode
	SupportedKinds   []Kind
	SupportedActions []Action
	Permissions      []PermissionScope
	RetryPolicy      RetryPolicy
}

// Object is the unified external data envelope across all seven connector domains.
// Every external record must carry its data source, external ID, update timestamp,
// version/revision, IANA timezone, and honest IsMock marker.
type Object struct {
	ID         string            // Canonical workbench ID ("<kind>:<external_id>"), populated on normalization.
	Source     string            // Upstream data source identifier (e.g. "caldav.work", "local.ics").
	ExternalID string            // Stable upstream identifier within Source.
	Kind       Kind              // One of the seven canonical kinds.
	UpdatedAt  time.Time         // Upstream modification timestamp.
	Version    string            // Upstream ETag, revision, or sequence string.
	Timezone   string            // IANA timezone name (e.g. "Asia/Shanghai").
	IsMock     bool              // True when the record comes from a synthetic/fixture source.
	Title      string            // Human-readable title or summary.
	Attributes map[string]string // Normalized domain-specific key-value attributes.
	Deleted    bool              // Tombstone marker for incremental sync deletions.
}

// CanonicalID returns the deterministic workbench object ID for a kind and external ID.
func CanonicalID(kind Kind, externalID string) string {
	return string(kind) + ":" + externalID
}

// Clone returns a deep copy of o.
func (o Object) Clone() Object {
	cp := o
	if cp.ID == "" && cp.Kind != "" && cp.ExternalID != "" {
		cp.ID = CanonicalID(cp.Kind, cp.ExternalID)
	}
	if len(o.Attributes) > 0 {
		cp.Attributes = make(map[string]string, len(o.Attributes))
		for k, v := range o.Attributes {
			cp.Attributes[k] = v
		}
	}
	return cp
}

// PageRequest specifies a paginated read query for a single kind.
type PageRequest struct {
	Kind        Kind
	PageSize    int
	PageToken   string
	RequireLive bool
}

// PageResult returns one page of canonical objects.
type PageResult struct {
	Objects       []Object
	NextPageToken string
	HasMore       bool
}

// SyncCursor is an opaque incremental sync token returned by a connector.
type SyncCursor string

// SyncRequest specifies an incremental sync operation.
type SyncRequest struct {
	Kind           Kind
	Cursor         SyncCursor
	PageSize       int
	IdempotencyKey string
	RequireLive    bool
}

// SyncResult contains a batch of incrementally synced objects and the next cursor.
type SyncResult struct {
	Objects        []Object
	NextCursor     SyncCursor
	HasMore        bool
	IdempotencyKey string
	SyncedAt       time.Time
}

// WriteRequest specifies a create/update/delete operation with an idempotency key.
type WriteRequest struct {
	Action         Action
	Object         Object
	IdempotencyKey string
	RequireLive    bool
}

// WriteResult reports the outcome of a write operation.
type WriteResult struct {
	Object         Object
	IdempotencyKey string
	Deduplicated   bool
}

// StatusReport describes the current connection health of a connector.
type StatusReport struct {
	ConnectorID string
	State       ConnectionState
	Mode        Mode
	CheckedAt   time.Time
	Detail      string
	RetryAfter  time.Duration
}

// Connector is the unified contract implemented by all external data adapters.
type Connector interface {
	Descriptor() Descriptor
	Status(ctx context.Context) (StatusReport, error)
	List(ctx context.Context, req PageRequest) (PageResult, error)
	Sync(ctx context.Context, req SyncRequest) (SyncResult, error)
	Write(ctx context.Context, req WriteRequest) (WriteResult, error)
}

// ErrorCode categorizes connector failures for diagnostics, retry decisions, and UI surfacing.
type ErrorCode string

const (
	CodeDisconnected        ErrorCode = "disconnected"
	CodeRateLimited         ErrorCode = "rate_limited"
	CodeUnauthorized        ErrorCode = "unauthorized"
	CodeMissingField        ErrorCode = "missing_field"
	CodeInvalidTimezone     ErrorCode = "invalid_timezone"
	CodeInvalidKind         ErrorCode = "invalid_kind"
	CodeMockAsLive          ErrorCode = "mock_as_live"
	CodePermissionViolation ErrorCode = "permission_violation"
	CodeUnsupportedAction   ErrorCode = "unsupported_action"
	CodeUnknownConnector    ErrorCode = "unknown_connector"
)

// Error is a structured, diagnosable connector error.
type Error struct {
	Code        ErrorCode
	ConnectorID string
	Field       string
	RetryAfter  time.Duration
	Message     string
	Cause       error
}

func (e *Error) Error() string {
	prefix := string(e.Code)
	if e.ConnectorID != "" {
		prefix = fmt.Sprintf("%s [%s]", e.Code, e.ConnectorID)
	}
	if e.Field != "" {
		prefix = fmt.Sprintf("%s (field=%s)", prefix, e.Field)
	}
	if e.Message != "" {
		return fmt.Sprintf("connector %s: %s", prefix, e.Message)
	}
	if e.Cause != nil {
		return fmt.Sprintf("connector %s: %v", prefix, e.Cause)
	}
	return "connector " + prefix
}

func (e *Error) Unwrap() error {
	return e.Cause
}

// IsCode reports whether err or any error in its chain has the given ErrorCode.
func IsCode(err error, code ErrorCode) bool {
	var ce *Error
	if errors.As(err, &ce) {
		return ce.Code == code
	}
	return false
}

// Disconnected constructs a diagnosable disconnected error.
func Disconnected(connectorID, msg string) error {
	return &Error{
		Code:        CodeDisconnected,
		ConnectorID: connectorID,
		Message:     msg,
	}
}

// RateLimited constructs a diagnosable rate-limit error with RetryAfter hint.
func RateLimited(connectorID string, retryAfter time.Duration, msg string) error {
	return &Error{
		Code:        CodeRateLimited,
		ConnectorID: connectorID,
		RetryAfter:  retryAfter,
		Message:     msg,
	}
}

// MissingField constructs a diagnosable missing-field validation error.
func MissingField(connectorID, field, msg string) error {
	return &Error{
		Code:        CodeMissingField,
		ConnectorID: connectorID,
		Field:       field,
		Message:     msg,
	}
}

// InvalidTimezone constructs a diagnosable timezone validation error.
func InvalidTimezone(connectorID, tz string, cause error) error {
	return &Error{
		Code:        CodeInvalidTimezone,
		ConnectorID: connectorID,
		Field:       "timezone",
		Message:     fmt.Sprintf("invalid IANA timezone %q", tz),
		Cause:       cause,
	}
}

// MockAsLive constructs an error rejecting a MOCK connector or object in a live context.
func MockAsLive(connectorID, msg string) error {
	return &Error{
		Code:        CodeMockAsLive,
		ConnectorID: connectorID,
		Message:     msg,
	}
}
