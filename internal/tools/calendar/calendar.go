package calendar

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/topsun-bot/proactive-workbench/internal/tool"
)

const ToolID = "calendar"

type Event struct {
	ID            string
	Title         string
	Start         time.Time
	Notes         string
	CreatedByFlow string
}

// Tool is an in-memory calendar stub. It does not talk to a real calendar.
type Tool struct {
	mu     sync.Mutex
	seq    int
	events []Event
}

func New() *Tool {
	return &Tool{}
}

func (t *Tool) Descriptor() tool.Descriptor {
	return tool.Descriptor{
		ID:          ToolID,
		DisplayName: "Calendar",
		Summary:     "In-memory calendar stub. Does not write to a system calendar yet.",
	}
}

func (t *Tool) Events() []Event {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]Event, len(t.events))
	copy(out, t.events)
	return out
}

func (t *Tool) Create(title string, start time.Time, notes, flow string) Event {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.seq++
	ev := Event{
		ID:            fmt.Sprintf("cal-%d", t.seq),
		Title:         title,
		Start:         start,
		Notes:         notes,
		CreatedByFlow: flow,
	}
	t.events = append(t.events, ev)
	return ev
}

func (t *Tool) Handle(req tool.Request) (tool.Result, error) {
	switch req.Action {
	case "createEvent":
		title := payload(req, "title")
		if title == "" {
			return tool.Result{}, tool.InvalidPayload("title is required")
		}
		start, err := time.Parse(time.RFC3339, payload(req, "start"))
		if err != nil {
			return tool.Result{}, tool.InvalidPayload("start must be RFC3339")
		}
		ev := t.Create(title, start, payload(req, "notes"), payload(req, "createdByFlow"))
		if ev.CreatedByFlow == "" {
			ev.CreatedByFlow = "plugin"
		}
		return tool.Result{
			Success: true,
			Summary: "Created calendar event “" + ev.Title + "”",
			Data: map[string]string{
				"id":    ev.ID,
				"title": ev.Title,
				"start": ev.Start.Format(time.RFC3339),
			},
		}, nil
	case "listEvents":
		events := t.Events()
		return tool.Result{
			Success: true,
			Summary: formatCount(len(events), "calendar event"),
			Data:    map[string]string{"count": strconv.Itoa(len(events))},
		}, nil
	default:
		return tool.Result{}, tool.Unsupported(req.Action)
	}
}

func payload(req tool.Request, key string) string {
	if req.Payload == nil {
		return ""
	}
	return req.Payload[key]
}

func formatCount(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(n) + " " + noun + "s"
}
