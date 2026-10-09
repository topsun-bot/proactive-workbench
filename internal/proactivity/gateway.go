package proactivity

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/topsun-bot/proactive-workbench/internal/memory"
)

const APIVersion = 1

// Handler serves the language-neutral JSON API. Bind only to loopback.
func Handler(core *Core) http.Handler {
	if core == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeErr(w, http.StatusInternalServerError, "proactivity core is required")
		})
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		writeOK(w, WireHealth{Service: "proactivity", API: APIVersion, Bound: "127.0.0.1"})
	})
	mux.HandleFunc("/v1/tick", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		res, err := core.Tick(r.Context())
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeOK(w, ResultToWire(res))
	})
	mux.HandleFunc("/v1/brief", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		b, err := core.Brief(r.Context())
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeOK(w, b)
	})
	mux.HandleFunc("/v1/memory", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		snap, err := core.Memory()
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeOK(w, snap)
	})
	mux.HandleFunc("/v1/memory/commitments", func(w http.ResponseWriter, r *http.Request) {
		handleUpsert(core, w, r, func(s *memory.Snapshot, body []byte) error {
			var c memory.Commitment
			if err := json.Unmarshal(body, &c); err != nil {
				return err
			}
			return s.UpsertCommitment(c)
		})
	})
	mux.HandleFunc("/v1/memory/people", func(w http.ResponseWriter, r *http.Request) {
		handleUpsert(core, w, r, func(s *memory.Snapshot, body []byte) error {
			var p memory.Person
			if err := json.Unmarshal(body, &p); err != nil {
				return err
			}
			return s.UpsertPerson(p)
		})
	})
	mux.HandleFunc("/v1/memory/preferences", func(w http.ResponseWriter, r *http.Request) {
		handleUpsert(core, w, r, func(s *memory.Snapshot, body []byte) error {
			var p memory.Preference
			if err := json.Unmarshal(body, &p); err != nil {
				return err
			}
			return s.UpsertPreference(p)
		})
	})
	mux.HandleFunc("/v1/memory/goals", func(w http.ResponseWriter, r *http.Request) {
		handleUpsert(core, w, r, func(s *memory.Snapshot, body []byte) error {
			var g memory.LongTermGoal
			if err := json.Unmarshal(body, &g); err != nil {
				return err
			}
			return s.UpsertGoal(g)
		})
	})
	mux.HandleFunc("/v1/memory/commitments/", func(w http.ResponseWriter, r *http.Request) {
		handleDelete(core, w, r, "/v1/memory/commitments/", func(s *memory.Snapshot, id string) bool {
			return s.DeleteCommitment(id)
		})
	})
	mux.HandleFunc("/v1/memory/people/", func(w http.ResponseWriter, r *http.Request) {
		handleDelete(core, w, r, "/v1/memory/people/", func(s *memory.Snapshot, id string) bool {
			return s.DeletePerson(id)
		})
	})
	mux.HandleFunc("/v1/memory/preferences/", func(w http.ResponseWriter, r *http.Request) {
		handleDelete(core, w, r, "/v1/memory/preferences/", func(s *memory.Snapshot, id string) bool {
			return s.DeletePreference(id)
		})
	})
	mux.HandleFunc("/v1/memory/goals/", func(w http.ResponseWriter, r *http.Request) {
		handleDelete(core, w, r, "/v1/memory/goals/", func(s *memory.Snapshot, id string) bool {
			return s.DeleteGoal(id)
		})
	})
	mux.HandleFunc("/v1/routines", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		st, err := core.Routines()
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeOK(w, st)
	})
	mux.HandleFunc("/v1/routines/run", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		runs, err := core.RunDue(r.Context())
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeOK(w, RoutineRunsToWire(runs))
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isLoopback(r.RemoteAddr) {
			writeErr(w, http.StatusForbidden, "proactivity API is loopback-only")
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func handleUpsert(core *Core, w http.ResponseWriter, r *http.Request, apply func(*memory.Snapshot, []byte) error) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	snap, err := core.UpdateMemory(func(s *memory.Snapshot) error {
		return apply(s, body)
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeOK(w, snap)
}

func handleDelete(core *Core, w http.ResponseWriter, r *http.Request, prefix string, del func(*memory.Snapshot, string) bool) {
	if r.Method != http.MethodDelete {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	id := strings.TrimPrefix(r.URL.Path, prefix)
	id = strings.Trim(id, "/")
	if id == "" {
		writeErr(w, http.StatusBadRequest, "id is required")
		return
	}
	snap, err := core.UpdateMemory(func(s *memory.Snapshot) error {
		if !del(s, id) {
			return fmt.Errorf("memory: %s not found", id)
		}
		return nil
	})
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	writeOK(w, snap)
}

func writeOK(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(Envelope{OK: true, Data: data})
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Envelope{OK: false, Error: msg})
}

func isLoopback(remote string) bool {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return host == "localhost"
	}
	return ip.IsLoopback()
}

func ListenAddr(addr string) (string, error) {
	if addr == "" {
		addr = "127.0.0.1:8741"
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("proactivity: listen: %w", err)
	}
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return "", fmt.Errorf("proactivity: listen host must be loopback, got %q", host)
	}
	if host == "localhost" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port), nil
}
