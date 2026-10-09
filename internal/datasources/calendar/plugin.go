package calendar

import (
	"context"
	"strconv"
	"time"

	"github.com/topsun-bot/proactive-workbench/internal/tool"
)

const ToolID = "calendar-datasource"

// ListTool exposes a read-only Source as Shaoruru’s tool.Tool (listEvents only).
// Creating events stays on the in-memory plugin until a write-capable backend exists.
type ListTool struct {
	src Source
}

func NewListTool(src Source) *ListTool {
	if src == nil {
		src = NewMock()
	}
	return &ListTool{src: src}
}

func (t *ListTool) Descriptor() tool.Descriptor {
	return tool.Descriptor{
		ID:          ToolID,
		DisplayName: "Calendar (data source)",
		Summary:     "Read events/reminders via internal/datasources/calendar (mock or ICS). Does not write.",
	}
}

func (t *ListTool) Handle(req tool.Request) (tool.Result, error) {
	if t == nil || t.src == nil {
		return tool.Result{}, tool.InvalidPayload("calendar source is not configured")
	}
	switch req.Action {
	case "listEvents":
		from, to, err := windowFromPayload(req.Payload)
		if err != nil {
			return tool.Result{}, err
		}
		events, err := t.src.ListEvents(context.Background(), Window{From: from, To: to})
		if err != nil {
			return tool.Result{}, err
		}
		return tool.Result{
			Success: true,
			Summary: formatCount(len(events), "calendar event"),
			Data:    map[string]string{"count": strconv.Itoa(len(events))},
		}, nil
	default:
		return tool.Result{}, tool.Unsupported(req.Action)
	}
}

func windowFromPayload(p map[string]string) (time.Time, time.Time, error) {
	if p == nil {
		return time.Time{}, time.Time{}, tool.InvalidPayload("from and to are required RFC3339 timestamps")
	}
	from, err := time.Parse(time.RFC3339, p["from"])
	if err != nil {
		return time.Time{}, time.Time{}, tool.InvalidPayload("from must be RFC3339")
	}
	to, err := time.Parse(time.RFC3339, p["to"])
	if err != nil {
		return time.Time{}, time.Time{}, tool.InvalidPayload("to must be RFC3339")
	}
	return from, to, nil
}

func formatCount(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(n) + " " + noun + "s"
}
