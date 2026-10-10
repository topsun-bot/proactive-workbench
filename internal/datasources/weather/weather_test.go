package weather_test

import (
	"bytes"
	"context"
	"embed"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/topsun-bot/proactive-workbench/internal/datasources/weather"
	"github.com/topsun-bot/proactive-workbench/internal/tool"
)

//go:embed fixtures/*.json
var testFixtures embed.FS

func shanghai() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		panic(err)
	}
	return loc
}

func TestConditionFromWMO(t *testing.T) {
	cases := []struct {
		code int
		want weather.Condition
	}{
		{0, weather.ConditionClear},
		{1, weather.ConditionClear},
		{2, weather.ConditionCloudy},
		{3, weather.ConditionCloudy},
		{45, weather.ConditionFog},
		{61, weather.ConditionRain},
		{80, weather.ConditionRain},
		{71, weather.ConditionSnow},
		{95, weather.ConditionStorm},
		{42, weather.ConditionUnknown},
	}
	for _, tc := range cases {
		if got := weather.ConditionFromWMO(tc.code); got != tc.want {
			t.Fatalf("code %d: got %s want %s", tc.code, got, tc.want)
		}
	}
}

func TestMockClearIsFixtureNotLive(t *testing.T) {
	src := weather.MustMock(weather.FixtureClear)
	at := time.Date(2026, 10, 10, 15, 0, 0, 0, shanghai())
	snap, err := src.Snapshot(context.Background(), weather.DefaultLocation, at)
	if err != nil {
		t.Fatal(err)
	}
	if !snap.IsMock {
		t.Fatal("fixture snapshot must set IsMock")
	}
	if snap.Source != weather.FixtureSourceLabel {
		t.Fatalf("source %q", snap.Source)
	}
	if !strings.Contains(snap.Source, "FIXTURE") {
		t.Fatalf("source must say FIXTURE: %s", snap.Source)
	}
	if snap.Current.Condition != weather.ConditionClear {
		t.Fatalf("condition %s", snap.Current.Condition)
	}
	if snap.Current.TemperatureC != 22.0 {
		t.Fatalf("fixture clear temp is the synthetic 22.0°C, got %v", snap.Current.TemperatureC)
	}
	if !weather.OutdoorComfort(snap.Current) {
		t.Fatal("clear fixture at 15:00 should be outdoor-OK")
	}
}

func TestMockRainTomorrowMorning(t *testing.T) {
	src := weather.MustMock(weather.FixtureRain)
	morning := time.Date(2026, 10, 10, 8, 0, 0, 0, shanghai())
	snap, err := src.Snapshot(context.Background(), weather.DefaultLocation, morning)
	if err != nil {
		t.Fatal(err)
	}
	p, ok := snap.PointAt(morning)
	if !ok {
		t.Fatal("missing hourly point")
	}
	if p.Condition != weather.ConditionRain {
		t.Fatalf("morning condition %s", p.Condition)
	}
	if !p.Condition.NeedsUmbrella() {
		t.Fatal("rain fixture must need an umbrella")
	}
	if p.TemperatureC != 16.0 {
		t.Fatalf("fixture rain temp is the synthetic 16.0°C, got %v", p.TemperatureC)
	}
}

func TestParseOpenMeteoRejectsSkewedArrays(t *testing.T) {
	body := []byte(`{"hourly":{"time":["2026-10-10T15:00"],"temperature_2m":[22],"weather_code":[0],"precipitation_probability":[0],"precipitation":[]}}`)
	_, err := weather.ParseOpenMeteoJSON(body, weather.DefaultLocation, time.UTC, true, weather.FixtureSourceLabel)
	if err == nil {
		t.Fatal("expected unequal-length error")
	}
}

func TestParseOpenMeteoFixtureFile(t *testing.T) {
	body, err := testFixtures.ReadFile("fixtures/openmeteo_cloudy.json")
	if err != nil {
		t.Fatal(err)
	}
	snap, err := weather.ParseOpenMeteoJSON(body, weather.DefaultLocation, shanghai(), true, weather.FixtureSourceLabel)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Current.Condition != weather.ConditionCloudy {
		t.Fatalf("condition %s", snap.Current.Condition)
	}
	if snap.Current.TemperatureC != 18.0 {
		t.Fatalf("fixture cloudy temp is the synthetic 18.0°C, got %v", snap.Current.TemperatureC)
	}
}

func TestOpenMeteoUsesInjectedDoerNoRealHost(t *testing.T) {
	body, err := testFixtures.ReadFile("fixtures/openmeteo_clear.json")
	if err != nil {
		t.Fatal(err)
	}
	src, err := weather.NewOpenMeteo(roundTripper{status: 200, body: body}, "http://weather.test.invalid")
	if err != nil {
		t.Fatal(err)
	}
	src.Timezone = "Asia/Shanghai"
	at := time.Date(2026, 10, 10, 15, 0, 0, 0, shanghai())
	snap, err := src.Snapshot(context.Background(), weather.DefaultLocation, at)
	if err != nil {
		t.Fatal(err)
	}
	if snap.IsMock {
		t.Fatal("HTTP adapter sets IsMock=false; fixture-ness is the injected body, labeled as open-meteo")
	}
	if snap.Source != weather.OpenMeteoSourceLabel {
		t.Fatalf("source %q", snap.Source)
	}
	if snap.Current.Condition != weather.ConditionClear {
		t.Fatalf("condition %s", snap.Current.Condition)
	}
}

func TestOpenMeteoRequiresDoer(t *testing.T) {
	if _, err := weather.NewOpenMeteo(nil, ""); err == nil {
		t.Fatal("nil Doer must be rejected so tests cannot accidentally dial out")
	}
}

func TestOpenMeteoHTTPError(t *testing.T) {
	src, err := weather.NewOpenMeteo(roundTripper{status: 503, body: []byte(`{"error":true}`)}, "http://weather.test.invalid")
	if err != nil {
		t.Fatal(err)
	}
	_, err = src.Snapshot(context.Background(), weather.DefaultLocation, time.Time{})
	if err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("expected HTTP 503, got %v", err)
	}
}

func TestPluginHandleMarksFixture(t *testing.T) {
	wrap := weather.NewTool(weather.MustMock(weather.FixtureClear))
	when := time.Date(2026, 10, 10, 15, 0, 0, 0, shanghai()).Format(time.RFC3339)
	res, err := wrap.Handle(tool.Request{Action: "forecast", Payload: map[string]string{"date": when}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Data["isMock"] != "true" {
		t.Fatalf("isMock=%q", res.Data["isMock"])
	}
	if !strings.Contains(res.Data["sourceLabel"], "FIXTURE") {
		t.Fatalf("source=%q", res.Data["sourceLabel"])
	}
	if res.Data["precipPct"] != "0" {
		t.Fatalf("expected precipPct=0 for clear fixture at 15:00, got %q", res.Data["precipPct"])
	}
	if res.Data["validFor"] == "" {
		t.Fatal("expected non-empty validFor in plugin result")
	}

	rainWrap := weather.NewTool(weather.MustMock(weather.FixtureRain))
	morning := time.Date(2026, 10, 10, 8, 0, 0, 0, shanghai()).Format(time.RFC3339)
	rainRes, err := rainWrap.Handle(tool.Request{Action: "forecast", Payload: map[string]string{"date": morning}})
	if err != nil {
		t.Fatal(err)
	}
	if rainRes.Data["precipPct"] != "80" {
		t.Fatalf("expected precipPct=80 for rain fixture at 08:00, got %q", rainRes.Data["precipPct"])
	}
}

func TestUnknownFixtureRejected(t *testing.T) {
	if _, err := weather.NewMock("hailstorm-invented"); err == nil {
		t.Fatal("expected error")
	}
}

type roundTripper struct {
	status int
	body   []byte
}

func (r roundTripper) Do(req *http.Request) (*http.Response, error) {
	if req.URL.Host != "weather.test.invalid" {
		return nil, netGuardError("refusing real network in unit test: " + req.URL.String())
	}
	status := r.status
	if status == 0 {
		status = 200
	}
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(bytes.NewReader(r.body)),
		Header:     make(http.Header),
		Request:    req,
	}, nil
}

type netGuardError string

func (e netGuardError) Error() string { return string(e) }
