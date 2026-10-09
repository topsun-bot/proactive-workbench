package today

import (
	"encoding/json"
	"net/http/httptest"
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
