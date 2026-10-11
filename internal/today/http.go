package today

import (
	"encoding/json"
	"io/fs"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/topsun-bot/proactive-workbench/internal/proactivity"
	"github.com/topsun-bot/proactive-workbench/internal/tools/weather"
)

func NewMux(now time.Time, loc *time.Location, wx weather.Condition) *http.ServeMux {
	return newMux(now, loc, wx, nil, false)
}

// NewDebugMux serves /api/today with labeled MOCK memory/people/task fixtures.
func NewDebugMux(now time.Time, loc *time.Location, wx weather.Condition) *http.ServeMux {
	return newMux(now, loc, wx, nil, true)
}

// NewMuxWithCore is NewMux plus an injectable Core. /api/today is the UI
// Snapshot. /v1/* is A4's wire contract. The core Handler's bare /api/today
// alias is not mounted here — that path stays Snapshot-only on workbench serve.
func NewMuxWithCore(now time.Time, loc *time.Location, wx weather.Condition, core *proactivity.Core) *http.ServeMux {
	return newMux(now, loc, wx, core, false)
}

func NewDebugMuxWithCore(now time.Time, loc *time.Location, wx weather.Condition, core *proactivity.Core) *http.ServeMux {
	return newMux(now, loc, wx, core, true)
}

func newMux(now time.Time, loc *time.Location, wx weather.Condition, core *proactivity.Core, debug bool) *http.ServeMux {
	if loc == nil {
		loc = time.UTC
	}
	if core == nil {
		var err error
		if debug {
			core, err = NewServeCore(now, loc, wx)
		} else {
			core, err = NewServeCoreNoMemory(now, loc, wx)
		}
		if err != nil {
			panic(err)
		}
	}
	live := now.IsZero()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/today", func(w http.ResponseWriter, r *http.Request) {
		stamp := now
		if live {
			stamp = time.Now().In(loc)
		}
		if raw := r.URL.Query().Get("now"); raw != "" {
			parsed, err := time.Parse(time.RFC3339, raw)
			if err == nil {
				stamp = parsed
			}
		}
		cond := wx
		if raw := r.URL.Query().Get("weather"); raw != "" {
			if parsed, ok := weather.ParseCondition(raw); ok {
				cond = parsed
			}
		}
		if stamp.Location() != loc {
			stamp = stamp.In(loc)
		}
		reqCore := core
		debugOn := debug || r.URL.Query().Get("debug-fixture") == "1"
		// Query overrides rebuild a Core. A live clock (now.IsZero at mux
		// creation) keeps the injected Core so Open-Meteo/ICS stay live.
		if r.URL.Query().Get("now") != "" || r.URL.Query().Get("weather") != "" {
			var built *proactivity.Core
			var err error
			if debugOn {
				built, err = NewServeCore(stamp, loc, cond)
			} else {
				built, err = NewServeCoreNoMemory(stamp, loc, cond)
			}
			if err == nil {
				reqCore = built
			}
		}
		mem := LongTerm{}
		if debugOn {
			mem = Fixture()
		}
		snap := Build(Input{Now: stamp, Weather: cond, Core: reqCore, DebugFixture: debugOn}, mem)
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(snap); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	if core != nil {
		// Mount only /v1/ so A4's Handler /api/today alias never shares this path.
		mux.Handle("/v1/", proactivity.Handler(core))
	}
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		panic(err)
	}
	mux.Handle("/", http.FileServer(http.FS(sub)))
	return mux
}

func ListenAndServe(addr string, now time.Time, loc *time.Location, wx weather.Condition) (string, *http.Server, string, error) {
	return ListenAndServeHandler(addr, NewMux(now, loc, wx))
}

func ListenAndServeDebug(addr string, now time.Time, loc *time.Location, wx weather.Condition) (string, *http.Server, string, error) {
	return ListenAndServeHandler(addr, NewDebugMux(now, loc, wx))
}

// ListenAndServeHandler binds the preferred loopback address and serves handler.
func ListenAndServeHandler(addr string, handler http.Handler) (string, *http.Server, string, error) {
	ln, err := ListenPreferred(addr)
	if err != nil {
		return "", nil, "", err
	}
	host, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		_ = ln.Close()
		return "", nil, "", err
	}
	if host == "::" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	if _, err := strconv.Atoi(port); err != nil {
		_ = ln.Close()
		return "", nil, "", err
	}
	portPath, err := WritePortFile(host, port)
	if err != nil {
		_ = ln.Close()
		return "", nil, "", err
	}
	srv := &http.Server{Handler: handler}
	go func() { _ = srv.Serve(ln) }()
	return "http://" + host + ":" + port + "/", srv, portPath, nil
}
