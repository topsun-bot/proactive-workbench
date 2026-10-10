package proactivity_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/topsun-bot/proactive-workbench/internal/clock"
	"github.com/topsun-bot/proactive-workbench/internal/datasources/calendar"
	"github.com/topsun-bot/proactive-workbench/internal/datasources/situation"
	"github.com/topsun-bot/proactive-workbench/internal/datasources/weather"
	"github.com/topsun-bot/proactive-workbench/internal/memory"
	"github.com/topsun-bot/proactive-workbench/internal/proactivity"
)

func testCore(t *testing.T) *proactivity.Core {
	t.Helper()
	store, err := memory.OpenFile(filepath.Join(t.TempDir(), "memory.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SeedFixture(); err != nil {
		t.Fatal(err)
	}
	s := sensors(
		weather.FixtureClear,
		situation.MustMock(situation.PlaceHome, situation.ActivityIdle),
		calendar.NewMock(),
		saturdayAfternoon(),
	)
	core, err := proactivity.NewCore(s, proactivity.DefaultPolicy(), store)
	if err != nil {
		t.Fatal(err)
	}
	return core
}

func loopbackReq(method, path string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, path, body)
	req.RemoteAddr = "127.0.0.1:1"
	return req
}

func decodeEnvelope(t *testing.T, rec *httptest.ResponseRecorder) (ok bool, errMsg string, data json.RawMessage) {
	t.Helper()
	var env struct {
		OK    bool            `json:"ok"`
		Error string          `json:"error"`
		Data  json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("envelope: %v body=%s", err, rec.Body.String())
	}
	return env.OK, env.Error, env.Data
}

func TestContractHealthTickBriefMemoryRoutines(t *testing.T) {
	h := proactivity.Handler(testCore(t))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, loopbackReq(http.MethodGet, "/v1/health", nil))
	ok, _, data := decodeEnvelope(t, rec)
	if rec.Code != 200 || !ok {
		t.Fatalf("health %d %s", rec.Code, rec.Body.String())
	}
	var health proactivity.WireHealth
	if err := json.Unmarshal(data, &health); err != nil {
		t.Fatal(err)
	}
	if health.Service != "proactivity" || health.API != 1 || health.Bound != "127.0.0.1" {
		t.Fatalf("%#v", health)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, loopbackReq(http.MethodPost, "/v1/tick", nil))
	ok, _, data = decodeEnvelope(t, rec)
	if !ok {
		t.Fatalf("tick %s", rec.Body.String())
	}
	var tick proactivity.WireTick
	if err := json.Unmarshal(data, &tick); err != nil {
		t.Fatal(err)
	}
	if tick.GoalKind != string(proactivity.GoalVisitPark) || !tick.Interrupt || !tick.Propose || !tick.SourcesAreFixtures {
		t.Fatalf("tick %#v", tick)
	}
	if tick.Reason == "" || tick.DecisionReason != tick.Reason {
		t.Fatalf("tick reason %#v", tick)
	}
	if _, err := time.Parse(time.RFC3339, tick.At); err != nil {
		t.Fatalf("at: %v", err)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, loopbackReq(http.MethodGet, "/v1/brief", nil))
	ok, _, data = decodeEnvelope(t, rec)
	if !ok {
		t.Fatal(rec.Body.String())
	}
	var brief proactivity.Brief
	if err := json.Unmarshal(data, &brief); err != nil {
		t.Fatal(err)
	}
	if !brief.IsFixture || !strings.Contains(brief.Source, "FIXTURE") {
		t.Fatalf("brief %#v", brief)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, loopbackReq(http.MethodGet, "/v1/memory", nil))
	ok, _, data = decodeEnvelope(t, rec)
	if !ok {
		t.Fatal(rec.Body.String())
	}
	var snap memory.Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		t.Fatal(err)
	}
	if snap.Version != memory.SchemaVersion || !snap.IsFixture {
		t.Fatalf("memory %#v", snap)
	}

	body := bytes.NewBufferString(`{"key":"likes_outdoors","value":"false","source":"contract-test"}`)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, loopbackReq(http.MethodPost, "/v1/memory/preferences", body))
	ok, _, _ = decodeEnvelope(t, rec)
	if !ok {
		t.Fatal(rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, loopbackReq(http.MethodGet, "/v1/routines", nil))
	ok, _, data = decodeEnvelope(t, rec)
	if !ok {
		t.Fatal(rec.Body.String())
	}
	var routines []proactivity.RoutineStatus
	if err := json.Unmarshal(data, &routines); err != nil {
		t.Fatal(err)
	}
	if len(routines) != 2 {
		t.Fatalf("routines %#v", routines)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, loopbackReq(http.MethodPost, "/v1/routines/run", nil))
	ok, _, data = decodeEnvelope(t, rec)
	if !ok {
		t.Fatal(rec.Body.String())
	}
	var runs []proactivity.WireRoutineRun
	if err := json.Unmarshal(data, &runs); err != nil {
		t.Fatal(err)
	}
	if len(runs) == 0 {
		t.Fatal("expected due routines on first run")
	}
}

func TestContractAPITodayProposeReason(t *testing.T) {
	h := proactivity.Handler(testCore(t))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, loopbackReq(http.MethodGet, "/api/today", nil))
	if rec.Code != 200 {
		t.Fatalf("code %d body %s", rec.Code, rec.Body.String())
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if _, wrapped := raw["ok"]; wrapped {
		t.Fatalf("/api/today must be bare JSON, not an envelope: %s", rec.Body.String())
	}
	var today proactivity.WireToday
	if err := json.Unmarshal(rec.Body.Bytes(), &today); err != nil {
		t.Fatal(err)
	}
	if len(today.Suggestions) != 1 {
		t.Fatalf("suggestions %#v", today.Suggestions)
	}
	sug := today.Suggestions[0]
	if !sug.Propose {
		t.Fatalf("park afternoon should propose: %#v", sug)
	}
	if sug.Reason == "" {
		t.Fatal("reason must be a non-empty string")
	}
	if !strings.Contains(today.LocationSource, "MOCK") {
		t.Fatalf("location must be labeled MOCK: %q", today.LocationSource)
	}

	rec = httptest.NewRecorder()
	proactivity.Handler(testCore(t)).ServeHTTP(rec, loopbackReq(http.MethodGet, "/v1/today", nil))
	ok, _, data := decodeEnvelope(t, rec)
	if !ok {
		t.Fatal(rec.Body.String())
	}
	var enveloped proactivity.WireToday
	if err := json.Unmarshal(data, &enveloped); err != nil {
		t.Fatal(err)
	}
	if len(enveloped.Suggestions) != 1 || !enveloped.Suggestions[0].Propose || enveloped.Suggestions[0].Reason == "" {
		t.Fatalf("/v1/today %#v", enveloped.Suggestions)
	}
}

func TestContractTodayQuietHoursProposeFalse(t *testing.T) {
	store, err := memory.OpenFile(filepath.Join(t.TempDir(), "memory.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SeedFixture(); err != nil {
		t.Fatal(err)
	}
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	s := sensors(
		weather.FixtureClear,
		situation.MustMock(situation.PlaceHome, situation.ActivityIdle),
		calendar.NewMock(),
		clock.Fixed(time.Date(2026, 10, 10, 23, 30, 0, 0, loc), loc),
	)
	core, err := proactivity.NewCore(s, proactivity.DefaultPolicy(), store)
	if err != nil {
		t.Fatal(err)
	}
	h := proactivity.Handler(core)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, loopbackReq(http.MethodGet, "/api/today", nil))
	var today proactivity.WireToday
	if err := json.Unmarshal(rec.Body.Bytes(), &today); err != nil {
		t.Fatal(err)
	}
	if len(today.Suggestions) != 1 {
		t.Fatalf("suggestions %#v", today.Suggestions)
	}
	if today.Suggestions[0].Propose {
		t.Fatalf("23:30 must not propose: %#v", today.Suggestions[0])
	}
	if !strings.Contains(today.Suggestions[0].Reason, "quiet hours") {
		t.Fatalf("reason %q", today.Suggestions[0].Reason)
	}
}

func TestContractRejectsNonLoopback(t *testing.T) {
	h := proactivity.Handler(testCore(t))
	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	req.RemoteAddr = "8.8.8.8:9"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("code %d body %s", rec.Code, rec.Body.String())
	}
	ok, errMsg, _ := decodeEnvelope(t, rec)
	if ok || !strings.Contains(errMsg, "loopback-only") {
		t.Fatalf("ok=%v err=%q", ok, errMsg)
	}
}

func TestListenAddrLoopbackOnly(t *testing.T) {
	if _, err := proactivity.ListenAddr("0.0.0.0:8741"); err == nil {
		t.Fatal("0.0.0.0 must be rejected")
	}
	got, err := proactivity.ListenAddr("localhost:8741")
	if err != nil {
		t.Fatal(err)
	}
	if got != "127.0.0.1:8741" {
		t.Fatalf("got %s", got)
	}
}

func TestJSONRoundTripEnvelope(t *testing.T) {
	raw, err := json.Marshal(proactivity.Envelope{OK: true, Data: proactivity.WireHealth{Service: "proactivity", API: 1, Bound: "127.0.0.1"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"ok":true`) || !strings.Contains(string(raw), `"api":1`) {
		t.Fatalf("%s", raw)
	}
	var env proactivity.Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	if !env.OK {
		t.Fatal(env)
	}
}
