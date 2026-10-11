package today

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/topsun-bot/proactive-workbench/internal/tools/weather"
)

func TestAPITodayReturnsSuggestion(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 10, 7, 15, 0, 0, loc)
	srv := httptest.NewServer(NewMux(now, loc, weather.Clear))
	t.Cleanup(srv.Close)
	res, err := srv.Client().Get(srv.URL + "/api/today")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status %d", res.StatusCode)
	}
	var snap Snapshot
	if err := json.NewDecoder(res.Body).Decode(&snap); err != nil {
		t.Fatal(err)
	}
	if len(snap.Suggestions) == 0 {
		t.Fatal("expected suggestions")
	}
}

func TestIndexIsServed(t *testing.T) {
	srv := httptest.NewServer(NewMux(time.Now(), time.UTC, weather.Clear))
	t.Cleanup(srv.Close)
	res, err := srv.Client().Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestAPITodayLiveClockIsNotTheFixtureDate(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(NewMux(time.Time{}, loc, ""))
	t.Cleanup(srv.Close)
	res, err := srv.Client().Get(srv.URL + "/api/today")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var snap Snapshot
	if err := json.NewDecoder(res.Body).Decode(&snap); err != nil {
		t.Fatal(err)
	}
	wantDate := time.Now().In(loc).Format("Monday, 2 January 2006")
	if snap.DateLabel != wantDate {
		t.Fatalf("DateLabel %q want live %q", snap.DateLabel, wantDate)
	}
	if strings.Contains(snap.WeatherLine, "Clear") || strings.Contains(snap.WeatherLine, "22°C") ||
		strings.Contains(snap.WeatherLine, "precip 5%") {
		t.Fatalf("live default leaked mock clear weather: %q", snap.WeatherLine)
	}
	if !strings.Contains(snap.WeatherLine, "天气暂时查不到") && !strings.Contains(snap.WeatherLine, "unavailable") {
		t.Fatalf("expected unavailable weather, got %q", snap.WeatherLine)
	}
	if snap.WeatherMock {
		t.Fatal("live default must not label weather as a MOCK scenario")
	}
}

func TestAPITodayDebugFixtureFreezesClockAndWeather(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	frozen := time.Date(2026, 10, 10, 7, 15, 0, 0, loc)
	srv := httptest.NewServer(NewMux(frozen, loc, weather.Clear))
	t.Cleanup(srv.Close)
	res, err := srv.Client().Get(srv.URL + "/api/today")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var snap Snapshot
	if err := json.NewDecoder(res.Body).Decode(&snap); err != nil {
		t.Fatal(err)
	}
	if snap.DateLabel != "Saturday, 10 October 2026" {
		t.Fatalf("fixture date %q", snap.DateLabel)
	}
	if !strings.Contains(snap.WeatherLine, "Clear") || !strings.Contains(snap.WeatherLine, "22°C") {
		t.Fatalf("fixture weather %q", snap.WeatherLine)
	}
}
