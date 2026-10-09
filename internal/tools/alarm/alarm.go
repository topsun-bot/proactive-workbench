package alarm

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/topsun-bot/proactive-workbench/internal/tool"
)

const ToolID = "alarm"

type Record struct {
	ID            string
	Label         string
	FireAt        time.Time
	CreatedByFlow string
}

// Tool is an in-memory alarm stub. It does not schedule a system alarm.
type Tool struct {
	mu     sync.Mutex
	seq    int
	alarms []Record
}

func New() *Tool {
	return &Tool{}
}

func (t *Tool) Descriptor() tool.Descriptor {
	return tool.Descriptor{
		ID:          ToolID,
		DisplayName: "Alarm",
		Summary:     "In-memory alarm stub. Does not schedule a system notification yet.",
	}
}

func (t *Tool) Alarms() []Record {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]Record, len(t.alarms))
	copy(out, t.alarms)
	return out
}

func (t *Tool) Create(label string, fireAt time.Time, flow string) Record {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.seq++
	rec := Record{
		ID:            fmt.Sprintf("alarm-%d", t.seq),
		Label:         label,
		FireAt:        fireAt,
		CreatedByFlow: flow,
	}
	t.alarms = append(t.alarms, rec)
	return rec
}

func (t *Tool) Handle(req tool.Request) (tool.Result, error) {
	switch req.Action {
	case "createAlarm":
		label := payload(req, "label")
		if label == "" {
			return tool.Result{}, tool.InvalidPayload("label is required")
		}
		fireAt, err := time.Parse(time.RFC3339, payload(req, "fireDate"))
		if err != nil {
			return tool.Result{}, tool.InvalidPayload("fireDate must be RFC3339")
		}
		flow := payload(req, "createdByFlow")
		if flow == "" {
			flow = "plugin"
		}
		rec := t.Create(label, fireAt, flow)
		return tool.Result{
			Success: true,
			Summary: "Set alarm “" + rec.Label + "”",
			Data: map[string]string{
				"id":            rec.ID,
				"label":         rec.Label,
				"fireDate":      rec.FireAt.Format(time.RFC3339),
				"createdByFlow": rec.CreatedByFlow,
			},
		}, nil
	case "listAlarms":
		alarms := t.Alarms()
		return tool.Result{
			Success: true,
			Summary: formatCount(len(alarms), "alarm"),
			Data:    map[string]string{"count": strconv.Itoa(len(alarms))},
		}, nil
	default:
		return tool.Result{}, tool.Unsupported(req.Action)
	}
}

// RecordFromResult rebuilds a Record from a plugin Handle("createAlarm") result.
func RecordFromResult(res tool.Result) (Record, error) {
	if res.Data == nil || res.Data["id"] == "" || res.Data["label"] == "" {
		return Record{}, tool.InvalidPayload("createAlarm result is incomplete")
	}
	fireAt, err := time.Parse(time.RFC3339, res.Data["fireDate"])
	if err != nil {
		return Record{}, tool.InvalidPayload("createAlarm result fireDate must be RFC3339")
	}
	return Record{
		ID:            res.Data["id"],
		Label:         res.Data["label"],
		FireAt:        fireAt,
		CreatedByFlow: res.Data["createdByFlow"],
	}, nil
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
