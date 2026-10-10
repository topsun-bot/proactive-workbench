package weather

import (
	"context"
	"strconv"
	"time"

	"github.com/topsun-bot/proactive-workbench/internal/tool"
)

const ToolID = "weather"

// Tool wraps a Source as Shaoruru’s tool.Tool so the existing registry can
// register a fixture or Open-Meteo backend without changing plugin code.
type Tool struct {
	src Source
	loc Location
}

func NewTool(src Source) *Tool {
	if src == nil {
		src = MustMock(FixtureClear)
	}
	return &Tool{src: src, loc: DefaultLocation}
}

func (t *Tool) SetLocation(loc Location) { t.loc = loc }

func (t *Tool) Descriptor() tool.Descriptor {
	return tool.Descriptor{
		ID:          ToolID,
		DisplayName: "Weather (data source)",
		Summary:     "Forecast via internal/datasources/weather (fixture or Open-Meteo). Not Shaoruru’s in-memory scenario stub.",
	}
}

func (t *Tool) Handle(req tool.Request) (tool.Result, error) {
	if t == nil || t.src == nil {
		return tool.Result{}, tool.InvalidPayload("weather source is not configured")
	}
	switch req.Action {
	case "forecast":
		when := time.Now()
		if raw, ok := req.Payload["date"]; ok && raw != "" {
			parsed, err := time.Parse(time.RFC3339, raw)
			if err != nil {
				return tool.Result{}, tool.InvalidPayload("date must be RFC3339")
			}
			when = parsed
		}
		snap, err := t.src.Snapshot(context.Background(), t.loc, when)
		if err != nil {
			return tool.Result{}, err
		}
		p, ok := snap.PointAt(when)
		if !ok {
			p = snap.Current
		}
		return tool.Result{
			Success: true,
			Summary: snap.Summary(),
			Data: map[string]string{
				"condition":    string(p.Condition),
				"temperatureC": strconv.Itoa(int(p.TemperatureC)),
				"precipPct":    strconv.Itoa(p.PrecipProbPct),
				"isMock":       strconv.FormatBool(snap.IsMock),
				"sourceLabel":  snap.Source,
				"location":     snap.Location.Label,
				"validFor":     p.At.Format(time.RFC3339),
			},
		}, nil
	default:
		return tool.Result{}, tool.Unsupported(req.Action)
	}
}
