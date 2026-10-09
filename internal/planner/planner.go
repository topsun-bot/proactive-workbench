package planner

import (
	"fmt"

	"github.com/topsun-bot/proactive-workbench/internal/clock"
	"github.com/topsun-bot/proactive-workbench/internal/flow/umbrella"
	"github.com/topsun-bot/proactive-workbench/internal/intent"
	"github.com/topsun-bot/proactive-workbench/internal/tools/alarm"
	"github.com/topsun-bot/proactive-workbench/internal/tools/calendar"
	"github.com/topsun-bot/proactive-workbench/internal/tools/weather"
)

type UnrecognizedGoalError struct {
	Goal string
}

func (e *UnrecognizedGoalError) Error() string {
	return fmt.Sprintf("no planner flow matches %q yet; try the umbrella reminder example", e.Goal)
}

// Plan routes a free-text goal to a cross-tool flow.
func Plan(goal string, w *weather.Tool, cal *calendar.Tool, al *alarm.Tool, clk clock.Clock) (umbrella.Plan, error) {
	switch intent.Recognize(goal).Kind {
	case intent.UmbrellaReminder:
		return umbrella.Execute(w, cal, al, clk)
	default:
		return umbrella.Plan{}, &UnrecognizedGoalError{Goal: goal}
	}
}
