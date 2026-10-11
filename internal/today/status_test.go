package today

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	dscal "github.com/topsun-bot/proactive-workbench/internal/datasources/calendar"
	dsweather "github.com/topsun-bot/proactive-workbench/internal/datasources/weather"
	"github.com/topsun-bot/proactive-workbench/internal/tools/weather"
)

func shanghai(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func assertNoFixtureText(t *testing.T, text string) {
	t.Helper()
	for _, leak := range []string{
		"standup with Sam",
		"Standup with Sam",
		"Morning walk",
		"Team standup",
		"Review umbrella demo PR",
		"sleep 4.5h",
		"4.5h last night",
		"Alex (roommate)",
	} {
		if strings.Contains(text, leak) {
			t.Fatalf("default mode leaked fixture %q\n%s", leak, text)
		}
	}
}

func TestDefaultModeHasNoFixtureContent(t *testing.T) {
	loc := shanghai(t)
	now := time.Date(2026, 10, 10, 7, 15, 0, 0, loc)
	core, err := NewServeCoreNoMemory(now, loc, weather.Unavailable)
	if err != nil {
		t.Fatal(err)
	}
	snap := Build(Input{Now: now, Weather: weather.Unavailable, Core: core, DebugFixture: false}, Fixture())
	text := Format(snap)
	assertNoFixtureText(t, text)
	if snap.MemorySource != "" {
		t.Fatalf("default MemorySource %q", snap.MemorySource)
	}
	if len(snap.Tasks) != 0 || len(snap.People) != 0 || len(snap.Routines) != 0 || len(snap.Suggestions) != 0 {
		t.Fatalf("default still has fixture cards: tasks=%d people=%d routines=%d suggestions=%d",
			len(snap.Tasks), len(snap.People), len(snap.Routines), len(snap.Suggestions))
	}
	if strings.Contains(text, "MOCK sleep") || strings.Contains(text, FixtureSourceLabel) {
		t.Fatalf("default labeled MOCK memory\n%s", text)
	}
}

func TestWeatherUnavailableUsesWeatherAvailableNotTempZero(t *testing.T) {
	loc := shanghai(t)
	now := time.Date(2026, 10, 11, 9, 0, 0, 0, loc)
	core, err := NewServeCoreNoMemory(now, loc, weather.Unavailable)
	if err != nil {
		t.Fatal(err)
	}
	snap := Build(Input{Now: now, Weather: weather.Unavailable, Core: core}, LongTerm{})
	if snap.WeatherAvailable {
		t.Fatal("WeatherAvailable must be false on a live miss")
	}
	if snap.WeatherUserMessage != dsweather.UserFacingUnavailable {
		t.Fatalf("WeatherUserMessage %q", snap.WeatherUserMessage)
	}
	if snap.WeatherLine != dsweather.UserFacingUnavailable {
		t.Fatalf("WeatherLine %q", snap.WeatherLine)
	}
	if snap.WeatherMock {
		t.Fatal("unavailable weather is not a MOCK scenario")
	}
	if strings.Contains(snap.WeatherLine, "0°C") || strings.Contains(Format(snap), "0°C") {
		t.Fatalf("client must not surface temp 0 as a reading\n%s", Format(snap))
	}
	if !strings.Contains(Format(snap), "天气暂时查不到") {
		t.Fatalf("missing 天气暂时查不到\n%s", Format(snap))
	}
}

func TestCalendarUnconfiguredStatus(t *testing.T) {
	loc := shanghai(t)
	now := time.Date(2026, 10, 11, 9, 0, 0, 0, loc)
	core, err := NewServeCoreNoMemory(now, loc, "")
	if err != nil {
		t.Fatal(err)
	}
	snap := Build(Input{Now: now, Core: core}, LongTerm{})
	if snap.CalendarStatus != CalendarStatusUnconfigured {
		t.Fatalf("CalendarStatus %q", snap.CalendarStatus)
	}
	if snap.CalendarAvailable {
		t.Fatal("unconfigured calendar must not be available")
	}
	if snap.CalendarUserMessage != dscal.UserFacingUnconfigured {
		t.Fatalf("CalendarUserMessage %q", snap.CalendarUserMessage)
	}
	if !strings.Contains(Format(snap), "日历未配置") {
		t.Fatalf("missing 日历未配置\n%s", Format(snap))
	}
	empty := ApplyEventKitStatus(snap, "granted", 0)
	if empty.CalendarStatus != CalendarStatusAvailable {
		t.Fatalf("granted empty list must stay available, got %q", empty.CalendarStatus)
	}
	if empty.CalendarUserMessage == dscal.UserFacingUnconfigured {
		t.Fatal("granted EventKit must hide 日历未配置 even with 0 events")
	}
}

func TestEventKitDeniedShowsPermissionDeniedNotUnconfigured(t *testing.T) {
	loc := shanghai(t)
	now := time.Date(2026, 10, 11, 9, 0, 0, 0, loc)
	core, err := NewServeCoreNoMemory(now, loc, "")
	if err != nil {
		t.Fatal(err)
	}
	snap := Build(Input{Now: now, Core: core}, LongTerm{})
	denied := ApplyEventKitStatus(snap, "denied", 0)
	if denied.CalendarStatus != CalendarStatusPermissionDenied {
		t.Fatalf("denied status %q", denied.CalendarStatus)
	}
	if denied.CalendarUserMessage != CalendarUserPermissionDenied {
		t.Fatalf("denied message %q", denied.CalendarUserMessage)
	}
	text := Format(denied)
	if !strings.Contains(text, "日历权限被拒绝") {
		t.Fatalf("missing permission-denied copy\n%s", text)
	}
	if strings.Contains(text, "日历未配置") {
		t.Fatalf("denied+no ICS must not say 未配置\n%s", text)
	}
}

func TestEventKitGrantedHidesUnconfigured(t *testing.T) {
	base := Snapshot{
		CalendarStatus:      CalendarStatusUnconfigured,
		CalendarAvailable:   false,
		CalendarUserMessage: dscal.UserFacingUnconfigured,
	}
	got := ApplyEventKitStatus(base, "granted", 2)
	if got.CalendarStatus != CalendarStatusAvailable {
		t.Fatalf("granted status %q", got.CalendarStatus)
	}
	if got.CalendarUserMessage != "" {
		t.Fatalf("granted should hide 未配置, got %q", got.CalendarUserMessage)
	}
	if !got.CalendarAvailable {
		t.Fatal("granted must be available")
	}
}

func TestDebugFixtureLabelsMOCK(t *testing.T) {
	loc := shanghai(t)
	now := time.Date(2026, 10, 10, 7, 15, 0, 0, loc)
	core, err := NewServeCore(now, loc, weather.Clear)
	if err != nil {
		t.Fatal(err)
	}
	text := Format(Build(Input{Now: now, Weather: weather.Clear, Core: core, DebugFixture: true}, Fixture()))
	for _, want := range []string{"Morning walk", "standup with Sam", "4.5h last night", "MOCK", "propose="} {
		if !strings.Contains(text, want) {
			t.Fatalf("debug fixture missing %q\n%s", want, text)
		}
	}
}

func TestAPITodayDefaultJSONHasNoFixtureText(t *testing.T) {
	loc := shanghai(t)
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
	raw, err := json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	assertNoFixtureText(t, string(raw))
	if snap.WeatherAvailable {
		t.Fatal("default mux weather must be unavailable without a live client")
	}
	if snap.CalendarStatus != CalendarStatusUnconfigured {
		t.Fatalf("default CalendarStatus %q", snap.CalendarStatus)
	}
	if snap.WeatherTempCImplicit() {
		t.Fatal("client must not treat a zero temp as available")
	}
}

// WeatherTempCImplicit is the forbidden client check: temp==0 is not a reading.
func (s Snapshot) WeatherTempCImplicit() bool {
	return !s.WeatherAvailable && strings.Contains(s.WeatherLine, "0°C")
}
