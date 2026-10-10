package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/topsun-bot/proactive-workbench/internal/clock"
	"github.com/topsun-bot/proactive-workbench/internal/datasources/calendar"
	"github.com/topsun-bot/proactive-workbench/internal/datasources/situation"
	"github.com/topsun-bot/proactive-workbench/internal/datasources/weather"
	"github.com/topsun-bot/proactive-workbench/internal/memory"
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
	case "brief":
		return cmdBrief(args[1:], stdout)
	case "memory":
		return cmdMemory(args[1:], stdout)
	case "serve":
		return cmdServe(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown command %q (try proactivity help)", args[0])
	}
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `Proactivity core — sense→goal→plan→interrupt, morning brief, local memory

Usage:
  proactivity tick [flags]
  proactivity brief [flags]
  proactivity memory show [flags]
  proactivity serve [flags]

Shared flags:
  --weather=clear|rain|cloudy   Fixture weather (not live data)
  --place=home|work|away        Fixture place (not GPS)
  --activity=idle|busy          Fixture activity
  --now=RFC3339                 Freeze the clock
  --tz=IANA                     Timezone (default Asia/Shanghai)
  --calendar-fixture            Load the labeled sample.ics fixture events
  --memory-file=PATH            Local JSON memory file
  --memory-fixture              Seed empty memory with the labeled fixture
  --json                        Language-neutral JSON envelope (same as HTTP)

tick extra:
  --repeat=N                    Run N ticks against the same dedupe store

serve extra:
  --listen=127.0.0.1:8741       Loopback only (see internal/proactivity/API.md)
  --last-run-file=PATH          Persist routine last-run JSON (default: platform path)

Weather, calendar, situation, and fixture memory are FIXTURES. This command
does not call Open-Meteo or any calendar host.
`)
}

type commonOpts struct {
	weather         string
	place           situation.Place
	activity        situation.Activity
	now             time.Time
	tz              *time.Location
	calendarFixture bool
	memoryFile      string
	memoryFixture   bool
	asJSON          bool
	repeat          int
	listen          string
	lastRunFile     string
}

func parseCommon(name string, args []string, extra func(*flag.FlagSet, *commonOpts)) (commonOpts, error) {
	opts := commonOpts{
		weather:  weather.FixtureClear,
		place:    situation.PlaceHome,
		activity: situation.ActivityIdle,
		repeat:   1,
		listen:   "127.0.0.1:8741",
	}
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	weatherFlag := fs.String("weather", weather.FixtureClear, "fixture weather")
	placeFlag := fs.String("place", "home", "fixture place")
	activityFlag := fs.String("activity", "idle", "fixture activity")
	nowFlag := fs.String("now", "", "override current time (RFC3339)")
	tzFlag := fs.String("tz", "Asia/Shanghai", "IANA timezone")
	calFix := fs.Bool("calendar-fixture", false, "load sample.ics")
	memFile := fs.String("memory-file", "", "local JSON memory")
	memFix := fs.Bool("memory-fixture", false, "seed labeled fixture memory")
	asJSON := fs.Bool("json", false, "JSON envelope")
	if extra != nil {
		extra(fs, &opts)
	}
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
	opts.weather = *weatherFlag
	opts.place = place
	opts.activity = act
	opts.tz = loc
	opts.calendarFixture = *calFix
	opts.memoryFile = *memFile
	opts.memoryFixture = *memFix
	opts.asJSON = *asJSON
	if *nowFlag != "" {
		parsed, err := time.Parse(time.RFC3339, *nowFlag)
		if err != nil {
			return opts, fmt.Errorf("invalid --now: %w", err)
		}
		opts.now = parsed
	}
	return opts, nil
}

func (opts commonOpts) sensors() (proactivity.Sensors, error) {
	wx, err := weather.NewMock(opts.weather)
	if err != nil {
		return proactivity.Sensors{}, err
	}
	wx.SetLocationTZ(opts.tz)
	sit, err := situation.NewMock(opts.place, opts.activity)
	if err != nil {
		return proactivity.Sensors{}, err
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
	return proactivity.Sensors{
		Weather:   wx,
		Calendar:  cal,
		Situation: sit,
		Clock:     clk,
		Location:  weather.DefaultLocation,
	}, nil
}

func (opts commonOpts) store() (memory.Store, error) {
	if opts.memoryFile == "" {
		ms := memory.NewMemStore()
		if opts.memoryFixture {
			if _, err := ms.SeedFixture(); err != nil {
				return nil, err
			}
		}
		return ms, nil
	}
	store, err := memory.OpenFile(opts.memoryFile)
	if err != nil {
		return nil, err
	}
	if opts.memoryFixture {
		if _, err := store.SeedFixture(); err != nil {
			return nil, err
		}
	}
	return store, nil
}

func writeJSON(w io.Writer, data any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(proactivity.Envelope{OK: true, Data: data})
}

func cmdTick(args []string, w io.Writer) error {
	opts, err := parseCommon("tick", args, func(fs *flag.FlagSet, o *commonOpts) {
		fs.IntVar(&o.repeat, "repeat", 1, "how many ticks to run")
	})
	if err != nil {
		return err
	}
	if opts.repeat < 1 || opts.repeat > 10 {
		return fmt.Errorf("--repeat must be 1–10")
	}
	sensors, err := opts.sensors()
	if err != nil {
		return err
	}
	store, err := opts.store()
	if err != nil {
		return err
	}
	core, err := proactivity.NewCore(sensors, proactivity.DefaultPolicy(), store)
	if err != nil {
		return err
	}
	if !opts.asJSON {
		fmt.Fprintln(w, "Proactive Workbench — proactivity core")
		fmt.Fprintln(w, "Sources are FIXTURES (not live weather, not a live calendar, not GPS).")
		fmt.Fprintln(w)
	}
	var last proactivity.Result
	for i := 1; i <= opts.repeat; i++ {
		res, err := core.Tick(context.Background())
		if err != nil {
			return err
		}
		last = res
		if opts.asJSON {
			continue
		}
		fmt.Fprint(w, proactivity.FormatResult(res, i))
		if i < opts.repeat {
			fmt.Fprintln(w)
		}
	}
	if opts.asJSON {
		return writeJSON(w, proactivity.ResultToWire(last))
	}
	return nil
}

func cmdBrief(args []string, w io.Writer) error {
	opts, err := parseCommon("brief", args, nil)
	if err != nil {
		return err
	}
	sensors, err := opts.sensors()
	if err != nil {
		return err
	}
	if opts.memoryFile == "" {
		opts.memoryFixture = true
	}
	store, err := opts.store()
	if err != nil {
		return err
	}
	core, err := proactivity.NewCore(sensors, proactivity.DefaultPolicy(), store)
	if err != nil {
		return err
	}
	b, err := core.Brief(context.Background())
	if err != nil {
		return err
	}
	if opts.asJSON {
		return writeJSON(w, b)
	}
	fmt.Fprint(w, proactivity.FormatBrief(b))
	return nil
}

func cmdMemory(args []string, w io.Writer) error {
	if len(args) == 0 || args[0] == "show" {
		if len(args) > 0 && args[0] == "show" {
			args = args[1:]
		}
		opts, err := parseCommon("memory", args, nil)
		if err != nil {
			return err
		}
		store, err := opts.store()
		if err != nil {
			return err
		}
		snap, err := store.Load()
		if err != nil {
			return err
		}
		if opts.asJSON {
			return writeJSON(w, snap)
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(snap)
	}
	return fmt.Errorf("unknown memory subcommand %q (try memory show)", args[0])
}

func cmdServe(args []string, stdout, stderr io.Writer) error {
	opts, err := parseCommon("serve", args, func(fs *flag.FlagSet, o *commonOpts) {
		fs.StringVar(&o.listen, "listen", "127.0.0.1:8741", "loopback address")
		fs.StringVar(&o.lastRunFile, "last-run-file", "", "routine last-run JSON (empty = platform default)")
	})
	if err != nil {
		return err
	}
	addr, err := proactivity.ListenAddr(opts.listen)
	if err != nil {
		return err
	}
	sensors, err := opts.sensors()
	if err != nil {
		return err
	}
	path := opts.memoryFile
	if path == "" {
		path = filepath.Join(os.TempDir(), "proactivity-serve-memory.json")
	}
	store, err := memory.OpenFile(path)
	if err != nil {
		return err
	}
	if opts.memoryFixture {
		if _, err := store.SeedFixture(); err != nil {
			return err
		}
	}
	lastRun := opts.lastRunFile
	if lastRun == "" {
		lastRun = proactivity.DefaultLastRunPath("", nil)
	}
	core, err := proactivity.NewCoreWithLastRun(sensors, proactivity.DefaultPolicy(), store, lastRun)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "proactivity API listening on http://%s (loopback only)\n", addr)
	fmt.Fprintf(stdout, "memory file: %s\n", store.Path())
	fmt.Fprintf(stdout, "last-run file: %s\n", lastRun)
	fmt.Fprintln(stdout, "GET /api/today is the Today alias (bare JSON; see API.md)")
	fmt.Fprintln(stdout, "see internal/proactivity/API.md")
	srv := &http.Server{
		Addr:              addr,
		Handler:           proactivity.Handler(core),
		ReadHeaderTimeout: 5 * time.Second,
	}
	_ = stderr
	return srv.ListenAndServe()
}
