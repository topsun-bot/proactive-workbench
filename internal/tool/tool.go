package tool

import (
	"fmt"
	"sort"
	"sync"
)

// Descriptor identifies a workbench plugin.
type Descriptor struct {
	ID          string
	DisplayName string
	Summary     string
}

// Request is the generic plugin call.
type Request struct {
	Action  string
	Payload map[string]string
}

// Result is the generic plugin response.
type Result struct {
	Success bool
	Summary string
	Data    map[string]string
}

// Tool is the plugin contract. Calendar, weather, alarm, and later tools
// implement this so the planner never hard-codes concrete types.
type Tool interface {
	Descriptor() Descriptor
	Handle(req Request) (Result, error)
}

// Error is a structured plugin failure.
type Error struct {
	Kind    string
	Message string
}

func (e *Error) Error() string {
	return e.Message
}

func Unsupported(action string) error {
	return &Error{Kind: "unsupported_action", Message: fmt.Sprintf("unsupported tool action: %s", action)}
}

func InvalidPayload(msg string) error {
	return &Error{Kind: "invalid_payload", Message: "invalid tool payload: " + msg}
}

// Registry holds plugins by id.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

func (r *Registry) Register(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[t.Descriptor().ID] = t
}

func (r *Registry) Get(id string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[id]
	return t, ok
}

func (r *Registry) Descriptors() []Descriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Descriptor, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t.Descriptor())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DisplayName < out[j].DisplayName })
	return out
}

func (r *Registry) Handle(toolID string, req Request) (Result, error) {
	t, ok := r.Get(toolID)
	if !ok {
		return Result{}, InvalidPayload("unknown tool id: " + toolID)
	}
	return t.Handle(req)
}
