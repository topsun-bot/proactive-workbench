package planner

import (
	"fmt"

	"github.com/topsun-bot/proactive-workbench/internal/clock"
	"github.com/topsun-bot/proactive-workbench/internal/flow/umbrella"
	"github.com/topsun-bot/proactive-workbench/internal/intent"
	"github.com/topsun-bot/proactive-workbench/internal/tool"
)

type UnrecognizedGoalError struct {
	Goal string
}

func (e *UnrecognizedGoalError) Error() string {
	return fmt.Sprintf("no planner flow matches %q yet; try the umbrella reminder example", e.Goal)
}

// Plan routes a free-text goal to a cross-tool flow. Tools are resolved
// from the plugin registry, not from concrete stub types.
func Plan(goal string, reg *tool.Registry, clk clock.Clock) (umbrella.Plan, error) {
	switch intent.Recognize(goal).Kind {
	case intent.UmbrellaReminder:
		return umbrella.Execute(reg, clk)
	default:
		return umbrella.Plan{}, &UnrecognizedGoalError{Goal: goal}
	}
}
