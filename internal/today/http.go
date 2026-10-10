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
	if loc == nil {
		loc = time.UTC
	}
	core, err := NewServeCore(now, loc, wx)
	if err != nil {
		panic(err)
	}
	return NewMuxWithCore(now, loc, wx, core)
}

// NewMuxWithCore is NewMux plus an injectable Core. /api/today is the UI
// Snapshot. /v1/* is A4's wire contract. The core Handler's bare /api/today
// alias is not mounted here — that path stays Snapshot-only on workbench serve.
func NewMuxWithCore(now time.Time, loc *time.Location, wx weather.Condition, core *proactivity.Core) *http.ServeMux {
	if loc == nil {
		loc = time.UTC
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
		if live || r.URL.Query().Get("now") != "" || r.URL.Query().Get("weather") != "" {
			built, err := NewServeCore(stamp, loc, cond)
			if err == nil {
				reqCore = built
			}
		}
		snap := Build(Input{Now: stamp, Weather: cond, Core: reqCore}, Fixture())
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
	srv := &http.Server{Handler: NewMux(now, loc, wx)}
	go func() { _ = srv.Serve(ln) }()
	return "http://" + host + ":" + port + "/", srv, portPath, nil
}
