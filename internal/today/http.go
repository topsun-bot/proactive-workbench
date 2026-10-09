package today

import (
	"encoding/json"
	"io/fs"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/topsun-bot/proactive-workbench/internal/memory"
	"github.com/topsun-bot/proactive-workbench/internal/tools/weather"
)

func NewMux(now time.Time, loc *time.Location, wx weather.Condition) *http.ServeMux {
	if loc == nil {
		loc = time.UTC
	}
	if wx == "" {
		wx = weather.Clear
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/today", func(w http.ResponseWriter, r *http.Request) {
		stamp := now
		if raw := r.URL.Query().Get("now"); raw != "" {
			parsed, err := time.Parse(time.RFC3339, raw)
			if err == nil {
				stamp = parsed
			}
		}
		cond := wx
		if raw := r.URL.Query().Get("weather"); raw != "" {
			if parsed, ok := weather.ParseCondition(raw); ok && parsed != weather.Unavailable {
				cond = parsed
			}
		}
		if stamp.Location() != loc {
			stamp = stamp.In(loc)
		}
		snap := Build(Input{Now: stamp, Weather: cond}, memory.Fixture())
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(snap); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		panic(err)
	}
	mux.Handle("/", http.FileServer(http.FS(sub)))
	return mux
}

func ListenAndServe(addr string, now time.Time, loc *time.Location, wx weather.Condition) (string, *http.Server, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return "", nil, err
	}
	srv := &http.Server{Handler: NewMux(now, loc, wx)}
	go func() { _ = srv.Serve(ln) }()
	host, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		_ = srv.Close()
		return "", nil, err
	}
	if host == "::" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	if _, err := strconv.Atoi(port); err != nil {
		_ = srv.Close()
		return "", nil, err
	}
	return "http://" + host + ":" + port + "/", srv, nil
}
