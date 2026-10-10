package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	_ "time/tzdata"

	"github.com/topsun-bot/proactive-workbench/internal/clock"
	"github.com/topsun-bot/proactive-workbench/internal/flow/umbrella"
	"github.com/topsun-bot/proactive-workbench/internal/planner"
	"github.com/topsun-bot/proactive-workbench/internal/tool"
	"github.com/topsun-bot/proactive-workbench/internal/tools/alarm"
	"github.com/topsun-bot/proactive-workbench/internal/tools/calendar"
	"github.com/topsun-bot/proactive-workbench/internal/tools/weather"
)

// Overridden at link time by scripts/package-linux.sh.
var version = "0.1.0"

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		printUsage(stdout)
		return nil
	}

	switch args[0] {
	case "version", "--version", "-v":
		fmt.Fprintf(stdout, "proactive-workbench %s linux\n", version)
		return nil
	case "help", "--help", "-h":
		printUsage(stdout)
		return nil
	case "tools":
		return cmdTools(stdout)
	case "demo":
		return cmdDemo(args[1:], stdout)
	case "plan":
		return cmdPlan(args[1:], stdout)
	default:
		return fmt.Errorf("unknown command %q (try workbench help)", args[0])
	}
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `Proactive Workbench — Linux CLI skeleton

Usage:
  workbench demo [--weather=rain|clear|cloudy|unavailable] [--now=RFC3339] [--tz=IANA]
  workbench plan "<goal>" [--weather=...] [--now=...] [--tz=...]
  workbench tools
  workbench version

The built-in cross-tool demo is:
  明天早上八点提醒带伞
  bring an umbrella tomorrow 8am

Weather in this build is MOCK (not live data). Per docs/PRD.md the
reminder is always created: rain, clear/cloudy, and weather-unavailable
use different copy. --weather=unavailable simulates a failed query.
`)
}

func cmdTools(w io.Writer) error {
	reg, _, _, _ := newSession(weather.Rain)
	fmt.Fprintln(w, "Registered tools")
	fmt.Fprintln(w, "----------------")
	for _, d := range reg.Descriptors() {
		fmt.Fprintf(w, "[%s] %s\n    %s\n", d.ID, d.DisplayName, d.Summary)
	}
	return nil
}

func cmdDemo(args []string, w io.Writer) error {
	opts, err := parseRunFlags(args, w)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return err
	}
	return executeGoal("明天早上八点提醒带伞", opts, w)
}

func cmdPlan(args []string, w io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("plan requires a goal string")
	}
	if args[0] == "-h" || args[0] == "--help" {
		_, err := parseRunFlags([]string{"-h"}, w)
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	goal := args[0]
	opts, err := parseRunFlags(args[1:], w)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return err
	}
	return executeGoal(goal, opts, w)
}

type runOpts struct {
	weather weather.Condition
	now     time.Time
	tz      *time.Location
}

func parseRunFlags(args []string, helpOut io.Writer) (runOpts, error) {
	opts := runOpts{weather: weather.Rain, tz: time.UTC}
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(helpOut)
	fs.Usage = func() {
		fmt.Fprint(helpOut, `Usage:
  workbench demo [--weather=rain|clear|cloudy|unavailable] [--now=RFC3339] [--tz=IANA]
  workbench plan "<goal>" [--weather=...] [--now=...] [--tz=...]

`)
		fs.PrintDefaults()
	}
	weatherFlag := fs.String("weather", "rain", "mock weather scenario: rain, clear, cloudy, unavailable")
	nowFlag := fs.String("now", "", "override current time (RFC3339); default is real now")
	tzFlag := fs.String("tz", "Asia/Shanghai", "IANA timezone for “tomorrow 8am”")
	if err := fs.Parse(args); err != nil {
		return opts, err
	}
	cond, ok := weather.ParseCondition(*weatherFlag)
	if !ok {
		return opts, fmt.Errorf("invalid --weather %q (use rain, clear, cloudy, or unavailable)", *weatherFlag)
	}
	opts.weather = cond
	loc, err := time.LoadLocation(*tzFlag)
	if err != nil {
		return opts, fmt.Errorf("invalid --tz: %w", err)
	}
	opts.tz = loc
	if *nowFlag != "" {
		parsed, err := time.Parse(time.RFC3339, *nowFlag)
		if err != nil {
			return opts, fmt.Errorf("invalid --now: %w", err)
		}
		opts.now = parsed
	}
	return opts, nil
}

func newSession(scenario weather.Condition) (*tool.Registry, *weather.Tool, *calendar.Tool, *alarm.Tool) {
	w := weather.New(scenario)
	cal := calendar.New()
	al := alarm.New()
	reg := tool.NewRegistry()
	reg.Register(w)
	reg.Register(cal)
	reg.Register(al)
	return reg, w, cal, al
}

func executeGoal(goal string, opts runOpts, w io.Writer) error {
	reg, _, _, _ := newSession(opts.weather)
	var clk clock.Clock
	if !opts.now.IsZero() {
		clk = clock.Fixed(opts.now, opts.tz)
	} else {
		clk = clock.Live(opts.tz)
	}

	fmt.Fprintln(w, "Proactive Workbench")
	fmt.Fprintln(w, "===================")
	fmt.Fprintf(w, "Goal:     %s\n", goal)
	fmt.Fprintf(w, "Weather:  %s  [%s]\n", opts.weather.DisplayName(), weather.SourceLabel)
	fmt.Fprintf(w, "Timezone: %s\n", opts.tz)
	if !opts.now.IsZero() {
		fmt.Fprintf(w, "Clock:    fixed %s\n", opts.now.In(opts.tz).Format(time.RFC3339))
	} else {
		fmt.Fprintf(w, "Clock:    live %s\n", clk.Now().In(opts.tz).Format(time.RFC3339))
	}
	fmt.Fprintln(w)

	plan, err := planner.Plan(goal, reg, clk)
	if err != nil {
		return err
	}
	printPlan(w, plan)
	return nil
}

func printPlan(w io.Writer, plan umbrella.Plan) {
	fmt.Fprintln(w, "Plan")
	fmt.Fprintln(w, "----")
	fmt.Fprintf(w, "Intent:   %s\n", plan.IntentID)
	fmt.Fprintf(w, "When:     %s\n", plan.ReminderAt.Format("2006-01-02 15:04 MST"))
	fmt.Fprintf(w, "Forecast: %s\n", plan.Weather.Summary())
	fmt.Fprintf(w, "isMock:   %v\n", plan.Weather.IsMock)
	if plan.Weather.Available {
		fmt.Fprintf(w, "Precip:   %d%% (threshold %d%%)\n", plan.Weather.PrecipPct, plan.RainThresholdPct)
	}
	fmt.Fprintf(w, "Outcome:  %s\n", plan.Outcome)
	fmt.Fprintf(w, "Message:  %s\n", plan.ReminderMessage)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Steps")
	fmt.Fprintln(w, "-----")
	for i, step := range plan.Steps {
		mark := "OK"
		if step.Status == umbrella.StepSkipped {
			mark = "SKIP"
		}
		fmt.Fprintf(w, "%d. [%s] %s (%s)\n   %s\n", i+1, mark, step.Title, step.ToolID, step.Detail)
	}
	fmt.Fprintln(w)
	if plan.CreatedReminders() {
		fmt.Fprintln(w, "Reminders created (in-memory stubs)")
		fmt.Fprintf(w, "  calendar: %s id=%s\n", plan.CalendarEvent.Title, plan.CalendarEvent.ID)
		fmt.Fprintf(w, "  alarm:    %s id=%s\n", plan.Alarm.Label, plan.Alarm.ID)
		fmt.Fprintf(w, "  message:  %s\n", plan.ReminderMessage)
	} else {
		fmt.Fprintln(w, "Reminders skipped")
		fmt.Fprintf(w, "  %s\n", plan.SkippedReason)
	}
}
