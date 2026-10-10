package today

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/topsun-bot/proactive-workbench/internal/proactivity"
	"github.com/topsun-bot/proactive-workbench/internal/tools/weather"
)

// macShouldShowBanner is the Mac gate 2 rule in macos/TodayWorkbench/gate.c:
// banner only when propose is present and true AND Focus is off.
func macShouldShowBanner(proposePresent, propose, focusOn bool) bool {
	if !proposePresent {
		return false
	}
	if !propose {
		return false
	}
	if focusOn {
		return false
	}
	return true
}

func TestCoreProposeReachesSnapshotUnchangedAndDrivesMacGate(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name    string
		now     time.Time
		wx      weather.Condition
		propose bool
	}{
		{
			name:    "quiet hours → propose false",
			now:     time.Date(2026, 10, 10, 7, 15, 0, 0, loc),
			wx:      weather.Clear,
			propose: false,
		},
		{
			name:    "afternoon park → propose true",
			now:     time.Date(2026, 10, 10, 15, 0, 0, 0, loc),
			wx:      weather.Clear,
			propose: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			core, err := NewServeCore(tc.now, loc, tc.wx)
			if err != nil {
				t.Fatal(err)
			}
			wire, err := core.Today(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if len(wire.Suggestions) == 0 {
				t.Fatal("core returned no suggestions")
			}
			got := wire.Suggestions[0]
			if got.Propose != tc.propose {
				t.Fatalf("core propose=%v want %v reason=%q", got.Propose, tc.propose, got.Reason)
			}

			// Fresh Core so Build's Today is the first tick (no dedupe).
			snapCore, err := NewServeCore(tc.now, loc, tc.wx)
			if err != nil {
				t.Fatal(err)
			}
			snap := Build(Input{Now: tc.now, Weather: tc.wx, Core: snapCore}, Fixture())
			if len(snap.Suggestions) != 1 {
				t.Fatalf("snapshot suggestions: %+v", snap.Suggestions)
			}
			s := snap.Suggestions[0]
			if s.Propose != got.Propose {
				t.Fatalf("snapshot propose=%v core=%v (recomputed?)", s.Propose, got.Propose)
			}
			if s.Reason != got.Reason {
				t.Fatalf("snapshot reason=%q core=%q (recomputed?)", s.Reason, got.Reason)
			}
			if s.Title != got.Title || s.Body != got.Body || s.Kind != got.Kind || s.Source != got.Source {
				t.Fatalf("snapshot suggestion %+v != core %+v", s, got)
			}

			raw, err := json.Marshal(snap)
			if err != nil {
				t.Fatal(err)
			}
			var payload map[string]json.RawMessage
			if err := json.Unmarshal(raw, &payload); err != nil {
				t.Fatal(err)
			}
			listRaw, ok := payload["Suggestions"]
			if !ok {
				t.Fatalf("Snapshot JSON missing Suggestions: %s", raw)
			}
			var items []map[string]any
			if err := json.Unmarshal(listRaw, &items); err != nil {
				t.Fatal(err)
			}
			if len(items) != 1 {
				t.Fatalf("json suggestions: %s", listRaw)
			}
			if items[0]["propose"] != got.Propose {
				t.Fatalf("json propose=%v want %v (%s)", items[0]["propose"], got.Propose, listRaw)
			}
			if items[0]["reason"] != got.Reason {
				t.Fatalf("json reason=%v want %q", items[0]["reason"], got.Reason)
			}

			// Mac gate 2: Focus off follows propose; Focus on always hides.
			if macShouldShowBanner(true, s.Propose, false) != s.Propose {
				t.Fatalf("Focus off: gate did not follow propose=%v", s.Propose)
			}
			if macShouldShowBanner(true, s.Propose, true) {
				t.Fatal("Focus on: Mac gate must hide even when propose=true")
			}
		})
	}
}

func TestAPITodayIsSnapshotAndV1TodayIsWire(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	// Quiet hours so two Today ticks (api + v1) both stay propose=false.
	now := time.Date(2026, 10, 10, 7, 15, 0, 0, loc)
	srv := httptest.NewServer(NewMux(now, loc, weather.Clear))
	t.Cleanup(srv.Close)

	apiRes, err := srv.Client().Get(srv.URL + "/api/today")
	if err != nil {
		t.Fatal(err)
	}
	defer apiRes.Body.Close()
	if apiRes.StatusCode != 200 {
		t.Fatalf("/api/today status %d", apiRes.StatusCode)
	}
	var apiRoot map[string]json.RawMessage
	if err := json.NewDecoder(apiRes.Body).Decode(&apiRoot); err != nil {
		t.Fatal(err)
	}
	if _, ok := apiRoot["ok"]; ok {
		t.Fatal("/api/today must be a bare Snapshot, not the /v1 envelope")
	}
	if _, ok := apiRoot["Greeting"]; !ok {
		t.Fatalf("/api/today missing Greeting: %v", keys(apiRoot))
	}
	if _, ok := apiRoot["at"]; ok {
		t.Fatal("/api/today must not be WireToday (unexpected at)")
	}
	var snap Snapshot
	rawAPI, err := json.Marshal(apiRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(rawAPI, &snap); err != nil {
		t.Fatal(err)
	}
	if snap.DateLabel != "Saturday, 10 October 2026" {
		t.Fatalf("snapshot DateLabel %q", snap.DateLabel)
	}
	if len(snap.Suggestions) == 0 {
		t.Fatal("/api/today Snapshot missing suggestions")
	}

	v1Res, err := srv.Client().Get(srv.URL + "/v1/today")
	if err != nil {
		t.Fatal(err)
	}
	defer v1Res.Body.Close()
	if v1Res.StatusCode != 200 {
		t.Fatalf("/v1/today status %d", v1Res.StatusCode)
	}
	var env struct {
		OK    bool                  `json:"ok"`
		Error string                `json:"error"`
		Data  proactivity.WireToday `json:"data"`
	}
	if err := json.NewDecoder(v1Res.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if !env.OK {
		t.Fatalf("/v1/today envelope not ok: %s", env.Error)
	}
	if env.Data.At == "" || env.Data.Place == "" {
		t.Fatalf("/v1/today missing WireToday fields: %+v", env.Data)
	}
	if len(env.Data.Suggestions) == 0 {
		t.Fatal("/v1/today missing suggestions")
	}
	if env.Data.Suggestions[0].Reason == "" {
		t.Fatal("wire suggestion missing reason")
	}
	// Re-read the raw envelope keys: /v1/today is {ok,data}, not Snapshot.
	v1Res2, err := srv.Client().Get(srv.URL + "/v1/today")
	if err != nil {
		t.Fatal(err)
	}
	defer v1Res2.Body.Close()
	var v1Root map[string]json.RawMessage
	if err := json.NewDecoder(v1Res2.Body).Decode(&v1Root); err != nil {
		t.Fatal(err)
	}
	if _, ok := v1Root["Greeting"]; ok {
		t.Fatal("/v1/today must be the envelope, not Snapshot")
	}
	if _, ok := v1Root["ok"]; !ok {
		t.Fatalf("/v1/today missing ok envelope: %v", keys(v1Root))
	}
}

func keys(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
