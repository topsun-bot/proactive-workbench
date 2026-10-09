package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/topsun-bot/proactive-workbench/internal/clock"
	"github.com/topsun-bot/proactive-workbench/internal/datasources/calendar"
	"github.com/topsun-bot/proactive-workbench/internal/datasources/situation"
	"github.com/topsun-bot/proactive-workbench/internal/datasources/weather"
	"github.com/topsun-bot/proactive-workbench/internal/proactivity"
)

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
	case "help", "--help", "-h":
		printUsage(stdout)
		return nil
	case "tick":
		return cmdTick(args[1:], stdout)
	default:
		return fmt.Errorf("unknown command %q (try proactivity help)", args[0])
	}
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `Proactivity core — one sense→goal→plan→interrupt tick (mocks only)

Usage:
  proactivity tick [flags]

Flags:
  --weather=clear|rain|cloudy   Fixture weather (not live data)
  --place=home|work|away        Fixture place (not GPS)
  --activity=idle|busy          Fixture activity
  --now=RFC3339                 Freeze the clock
  --tz=IANA                     Timezone (default Asia/Shanghai)
  --repeat=N                    Run N ticks against the same memory (dedupe demo)
  --calendar-fixture            Load the labeled sample.ics fixture events

Weather, calendar, and situation are FIXTURES. This command does not call
Open-Meteo or any calendar host. See internal/datasources/DESIGN.md.
`)
}

type tickOpts struct {
	weather         string
	place           situation.Place
	activity        situation.Activity
	now             time.Time
	tz              *time.Location
	repeat          int
	calendarFixture bool
}

func parseTickFlags(args []string) (tickOpts, error) {
	opts := tickOpts{weather: weather.FixtureClear, place: situation.PlaceHome, activity: situation.ActivityIdle, repeat: 1}
	fs := flag.NewFlagSet("tick", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	weatherFlag := fs.String("weather", weather.FixtureClear, "fixture weather: clear, rain, cloudy")
	placeFlag := fs.String("place", "home", "fixture place")
	activityFlag := fs.String("activity", "idle", "fixture activity")
	nowFlag := fs.String("now", "", "override current time (RFC3339)")
	tzFlag := fs.String("tz", "Asia/Shanghai", "IANA timezone")
	repeatFlag := fs.Int("repeat", 1, "how many ticks to run")
	calFix := fs.Bool("calendar-fixture", false, "load sample.ics fixture events")
	if err := fs.Parse(args); err != nil {
		return opts, err
	}
	if _, ok := weather.ParseCondition(*weatherFlag); !ok {
		return opts, fmt.Errorf("invalid --weather %q (use clear, rain, or cloudy)", *weatherFlag)
	}
	place, ok := situation.ParsePlace(*placeFlag)
	if !ok {
		return opts, fmt.Errorf("invalid --place %q", *placeFlag)
	}
	act, ok := situation.ParseActivity(*activityFlag)
	if !ok {
		return opts, fmt.Errorf("invalid --activity %q", *activityFlag)
	}
	loc, err := time.LoadLocation(*tzFlag)
	if err != nil {
		return opts, fmt.Errorf("invalid --tz: %w", err)
	}
	if *repeatFlag < 1 || *repeatFlag > 10 {
		return opts, fmt.Errorf("--repeat must be 1–10")
	}
	opts.weather = *weatherFlag
	opts.place = place
	opts.activity = act
	opts.tz = loc
	opts.repeat = *repeatFlag
	opts.calendarFixture = *calFix
	if *nowFlag != "" {
		parsed, err := time.Parse(time.RFC3339, *nowFlag)
		if err != nil {
			return opts, fmt.Errorf("invalid --now: %w", err)
		}
		opts.now = parsed
	}
	return opts, nil
}

func cmdTick(args []string, w io.Writer) error {
	opts, err := parseTickFlags(args)
	if err != nil {
		return err
	}

	wx, err := weather.NewMock(opts.weather)
	if err != nil {
		return err
	}
	wx.SetLocationTZ(opts.tz)

	sit, err := situation.NewMock(opts.place, opts.activity)
	if err != nil {
		return err
	}

	var cal calendar.Source
	if opts.calendarFixture {
		cal = calendar.MustFixtureMock()
	} else {
		cal = calendar.NewMock()
	}

	var clk clock.Clock
	if !opts.now.IsZero() {
		clk = clock.Fixed(opts.now, opts.tz)
	} else {
		clk = clock.Live(opts.tz)
	}

	sensors := proactivity.Sensors{
		Weather:   wx,
		Calendar:  cal,
		Situation: sit,
		Clock:     clk,
		Location:  weather.DefaultLocation,
	}
	mem := proactivity.NewMemory()
	policy := proactivity.DefaultPolicy()

	fmt.Fprintln(w, "Proactive Workbench — proactivity core")
	fmt.Fprintln(w, "Sources are FIXTURES (not live weather, not a live calendar, not GPS).")
	fmt.Fprintln(w)

	for i := 1; i <= opts.repeat; i++ {
		res, err := proactivity.Tick(context.Background(), sensors, policy, mem)
		if err != nil {
			return err
		}
		fmt.Fprint(w, proactivity.FormatResult(res, i))
		if i < opts.repeat {
			fmt.Fprintln(w)
		}
	}
	return nil
}
