package appconfig_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/topsun-bot/proactive-workbench/internal/appconfig"
	"github.com/topsun-bot/proactive-workbench/internal/datasources/calendar"
	"github.com/topsun-bot/proactive-workbench/internal/datasources/weather"
	"github.com/topsun-bot/proactive-workbench/internal/proactivity"
)

func shanghai() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		panic(err)
	}
	return loc
}

type roundTrip struct {
	status int
	body   []byte
	err    error
	host   string
}

func (r roundTrip) Do(req *http.Request) (*http.Response, error) {
	if r.host != "" && req.URL.Host != r.host {
		return nil, errors.New("refusing real network in unit test: " + req.URL.String())
	}
	if r.err != nil {
		return nil, r.err
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

func TestSensorsLiveSuccess(t *testing.T) {
	body := []byte(`{"latitude":31.22,"longitude":121.42,"timezone":"Asia/Shanghai","current":{"time":"2026-10-10T15:00","temperature_2m":21.5,"weather_code":0,"precipitation":0},"hourly":{"time":["2026-10-10T15:00"],"temperature_2m":[21.5],"weather_code":[0],"precipitation_probability":[5],"precipitation":[0]}}`)
	s, err := appconfig.Sensors(appconfig.SensorOpts{
		Config:         appconfig.Config{Timezone: "Asia/Shanghai"},
		Now:            time.Date(2026, 10, 10, 15, 0, 0, 0, shanghai()),
		TZ:             shanghai(),
		WeatherDoer:    roundTrip{status: 200, body: body, host: "weather.test.invalid"},
		WeatherBaseURL: "http://weather.test.invalid",
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := proactivity.Perceive(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Weather.Available() || p.Weather.IsMock {
		t.Fatalf("live success must be available and not mock: %#v", p.Weather)
	}
	if p.NowWeather.TemperatureC != 21.5 {
		t.Fatalf("temp %v", p.NowWeather.TemperatureC)
	}
	if p.CalendarError == "" {
		t.Fatal("missing ICS config must set CalendarError")
	}
	if p.CalendarUserMessage != calendar.UserFacingUnconfigured {
		t.Fatalf("cal msg %q", p.CalendarUserMessage)
	}
	if len(p.Events) != 0 {
		t.Fatalf("invented events: %#v", p.Events)
	}
}

func TestSensorsNetworkFailureNoFabricatedWeather(t *testing.T) {
	s, err := appconfig.Sensors(appconfig.SensorOpts{
		Config:         appconfig.Config{Timezone: "Asia/Shanghai"},
		Now:            time.Date(2026, 10, 10, 15, 0, 0, 0, shanghai()),
		TZ:             shanghai(),
		WeatherDoer:    roundTrip{err: errors.New("dial tcp: no route"), host: "weather.test.invalid"},
		WeatherBaseURL: "http://weather.test.invalid",
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := proactivity.Perceive(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	if p.Weather.Available() {
		t.Fatal("failed fetch must be unavailable")
	}
	if p.NowWeather.TemperatureC != 0 || p.NowWeather.PrecipProbPct != 0 {
		t.Fatalf("fabricated numbers: %#v", p.NowWeather)
	}
	if p.NowWeather.Condition != weather.ConditionUnavailable {
		t.Fatalf("condition %s", p.NowWeather.Condition)
	}
	if !strings.Contains(p.Weather.Error, "no route") {
		t.Fatalf("error %q", p.Weather.Error)
	}
	wire := proactivity.ResultToWire(proactivity.Result{Perception: p})
	if wire.WeatherAvailable || wire.WeatherUserMessage != weather.UserFacingUnavailable {
		t.Fatalf("wire %#v", wire)
	}
	if wire.WeatherTempC != 0 {
		t.Fatalf("wire fabricated temp %v", wire.WeatherTempC)
	}
	text := proactivity.FormatResult(proactivity.Result{Perception: p}, 1)
	if !strings.Contains(text, weather.UserFacingUnavailable) {
		t.Fatalf("cli:\n%s", text)
	}
	if strings.Contains(text, "22.0°C") || strings.Contains(text, "16.0°C") {
		t.Fatalf("fixture temps leaked:\n%s", text)
	}
}

func TestSensorsMissingICS(t *testing.T) {
	s, err := appconfig.Sensors(appconfig.SensorOpts{
		Config:         appconfig.Config{Timezone: "Asia/Shanghai"},
		Now:            time.Date(2026, 10, 10, 15, 0, 0, 0, shanghai()),
		TZ:             shanghai(),
		WeatherDoer:    roundTrip{status: 503, body: []byte(`{}`), host: "weather.test.invalid"},
		WeatherBaseURL: "http://weather.test.invalid",
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := proactivity.Perceive(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	if p.CalendarError == "" || p.CalendarUserMessage != calendar.UserFacingUnconfigured {
		t.Fatalf("cal %#v / %q", p.CalendarError, p.CalendarUserMessage)
	}
	wire := proactivity.ResultToToday(proactivity.Result{Perception: p}, proactivity.Brief{})
	if wire.CalendarAvailable || wire.CalendarUserMessage != calendar.UserFacingUnconfigured {
		t.Fatalf("today wire %#v", wire)
	}
}

func TestSensorsUnreadableICS(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such", "missing.ics")
	s, err := appconfig.Sensors(appconfig.SensorOpts{
		Config:         appconfig.Config{Timezone: "Asia/Shanghai", ICSPath: path},
		Now:            time.Date(2026, 10, 10, 15, 0, 0, 0, shanghai()),
		TZ:             shanghai(),
		WeatherDoer:    roundTrip{status: 503, body: []byte(`{}`), host: "weather.test.invalid"},
		WeatherBaseURL: "http://weather.test.invalid",
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := proactivity.Perceive(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	if p.CalendarUserMessage != calendar.UserFacingUnreadable {
		t.Fatalf("msg %q err %q", p.CalendarUserMessage, p.CalendarError)
	}
	if len(p.Events) != 0 {
		t.Fatalf("invented events %#v", p.Events)
	}
}

func TestSensorsDebugFixture(t *testing.T) {
	s, err := appconfig.Sensors(appconfig.SensorOpts{
		DebugFixture:   true,
		WeatherFixture: weather.FixtureClear,
		Now:            time.Date(2026, 10, 10, 15, 0, 0, 0, shanghai()),
		TZ:             shanghai(),
		// If fixtures leaked to live, this doer would be unused; a real
		// DefaultClient must not be constructed. Doer is ignored in fixture mode.
		WeatherDoer: roundTrip{err: errors.New("should not dial"), host: "weather.test.invalid"},
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := proactivity.Perceive(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Weather.IsMock || !strings.Contains(p.Weather.Source, "FIXTURE") {
		t.Fatalf("fixture labels %#v", p.Weather)
	}
	if p.NowWeather.TemperatureC != 22.0 {
		t.Fatalf("fixture clear temp %v", p.NowWeather.TemperatureC)
	}
	if p.CalendarError != "" {
		t.Fatalf("empty fixture calendar should be available: %s", p.CalendarError)
	}
}

func TestSensorsLiveICSFile(t *testing.T) {
	src := filepath.Join("..", "datasources", "calendar", "fixtures", "sample.ics")
	body, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "copy.ics")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	wxBody := []byte(`{"latitude":31.22,"longitude":121.42,"timezone":"Asia/Shanghai","current":{"time":"2026-10-09T14:00","temperature_2m":18,"weather_code":3,"precipitation":0},"hourly":{"time":["2026-10-09T14:00"],"temperature_2m":[18],"weather_code":[3],"precipitation_probability":[10],"precipitation":[0]}}`)
	s, err := appconfig.Sensors(appconfig.SensorOpts{
		Config:         appconfig.Config{Timezone: "Asia/Shanghai", ICSPath: path},
		Now:            time.Date(2026, 10, 9, 13, 0, 0, 0, shanghai()),
		TZ:             shanghai(),
		WeatherDoer:    roundTrip{status: 200, body: wxBody, host: "weather.test.invalid"},
		WeatherBaseURL: "http://weather.test.invalid",
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := proactivity.Perceive(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	if p.CalendarError != "" {
		t.Fatalf("ics error %s", p.CalendarError)
	}
	if len(p.Events) == 0 {
		t.Fatal("expected fixture event from the copied ICS (not invented)")
	}
}
