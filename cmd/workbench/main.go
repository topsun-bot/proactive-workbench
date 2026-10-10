package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	_ "time/tzdata"

	"github.com/topsun-bot/proactive-workbench/internal/clock"
	"github.com/topsun-bot/proactive-workbench/internal/flow/umbrella"
	"github.com/topsun-bot/proactive-workbench/internal/planner"
	"github.com/topsun-bot/proactive-workbench/internal/today"
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
		fmt.Fprintf(stdout, "proactive-workbench %s %s\n", version, runtime.GOOS)
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
	case "today":
		return cmdToday(args[1:], stdout)
	case "serve":
		return cmdServe(args[1:], stdout)
	default:
		return fmt.Errorf("unknown command %q (try workbench help)", args[0])
	}
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `Proactive Workbench — shared Go core (Linux CLI + Mac Today UI)

Usage:
  workbench demo [--weather=rain|clear|cloudy|unavailable] [--now=RFC3339] [--tz=IANA]
  workbench plan "<goal>" [--weather=...] [--now=...] [--tz=...]
  workbench today [--weather=...] [--now=...] [--tz=...]
  workbench serve [--addr=127.0.0.1:8741] [--debug-fixture] [--weather=...] [--now=...] [--tz=...]
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

func cmdToday(args []string, w io.Writer) error {
	opts, err := parseRunFlags(args, w)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return err
	}
	now := opts.now
	if now.IsZero() {
		now = time.Now().In(opts.tz)
	} else {
		now = now.In(opts.tz)
	}
	wx := opts.weather
	core, err := today.NewServeCore(now, opts.tz, wx)
	if err != nil {
		return err
	}
	fmt.Fprint(w, today.Format(today.Build(today.Input{Now: now, Weather: wx, Core: core}, today.Fixture())))
	return nil
}

const (
	debugFixtureNow     = "2026-10-10T07:15:00+08:00"
	debugFixtureWeather = "clear"
)

type serveOpts struct {
	addr       string
	weather    weather.Condition
	weatherSet bool
	now        time.Time
	tz         *time.Location
	debugOn    bool
}

func envTruthy(key string) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return false
	}
	switch strings.ToLower(v) {
	case "0", "false", "off", "no":
		return false
	default:
		return true
	}
}

func parseServeArgs(args []string, helpOut io.Writer) (serveOpts, error) {
	opts := serveOpts{addr: today.DefaultListenAddr, tz: time.UTC}
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(helpOut)
	fs.Usage = func() {
		fmt.Fprint(helpOut, `Usage:
  workbench serve [--addr=127.0.0.1:8741] [--debug-fixture] [--weather=...] [--now=RFC3339] [--tz=IANA]

Default: real current time and no MOCK weather scenario.
Mock clock/weather only via --debug-fixture, --now, --weather, or env
PW_DEBUG_FIXTURE / PW_DEBUG_NOW / PW_DEBUG_WEATHER.

`)
		fs.PrintDefaults()
	}
	addrFlag := fs.String("addr", today.DefaultListenAddr, "listen address (falls back if busy)")
	weatherFlag := fs.String("weather", "", "MOCK weather scenario (debug only)")
	nowFlag := fs.String("now", "", "override current time RFC3339 (debug only)")
	tzFlag := fs.String("tz", "Asia/Shanghai", "IANA timezone")
	debugFlag := fs.Bool("debug-fixture", false, "use the canned 2026-10-10 07:15 + clear MOCK weather")
	if err := fs.Parse(args); err != nil {
		return opts, err
	}
	opts.addr = *addrFlag
	opts.debugOn = *debugFlag || envTruthy("PW_DEBUG_FIXTURE")

	nowRaw := *nowFlag
	if nowRaw == "" {
		nowRaw = strings.TrimSpace(os.Getenv("PW_DEBUG_NOW"))
	}
	wxRaw := *weatherFlag
	if wxRaw == "" {
		wxRaw = strings.TrimSpace(os.Getenv("PW_DEBUG_WEATHER"))
	}
	if opts.debugOn {
		if nowRaw == "" {
			nowRaw = debugFixtureNow
		}
		if wxRaw == "" {
			wxRaw = debugFixtureWeather
		}
	}

	loc, err := time.LoadLocation(*tzFlag)
	if err != nil {
		return opts, fmt.Errorf("invalid --tz: %w", err)
	}
	opts.tz = loc
	if nowRaw != "" {
		parsed, err := time.Parse(time.RFC3339, nowRaw)
		if err != nil {
			return opts, fmt.Errorf("invalid --now: %w", err)
		}
		opts.now = parsed.In(loc)
	}
	if wxRaw != "" {
		cond, ok := weather.ParseCondition(wxRaw)
		if !ok {
			return opts, fmt.Errorf("invalid --weather %q (use rain, clear, cloudy, or unavailable)", wxRaw)
		}
		opts.weather = cond
		opts.weatherSet = true
	}
	return opts, nil
}

func cmdServe(args []string, w io.Writer) error {
	opts, err := parseServeArgs(args, w)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return err
	}
	url, srv, portPath, err := today.ListenAndServe(opts.addr, opts.now, opts.tz, opts.weather)
	if err != nil {
		return err
	}
	// First stdout line is the URL (Mac shell parses it). Port path goes to stderr.
	fmt.Fprintln(w, url)
	if portPath != "" {
		fmt.Fprintf(os.Stderr, "port-file: %s\n", portPath)
	}
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	return srv.Close()
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
