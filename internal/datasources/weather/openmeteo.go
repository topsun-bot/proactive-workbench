package weather

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const DefaultOpenMeteoBaseURL = "https://api.open-meteo.com"

// Doer is the HTTP seam. Unit tests inject a fake; live code may pass
// http.DefaultClient. The adapter never falls back to a real client on its own.
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

// OpenMeteo is the keyless forecast adapter.
type OpenMeteo struct {
	BaseURL  string
	Doer     Doer
	Timezone string
}

// NewOpenMeteo builds an adapter. doer must be non-nil; tests must pass a fake.
func NewOpenMeteo(doer Doer, baseURL string) (*OpenMeteo, error) {
	if doer == nil {
		return nil, fmt.Errorf("weather: Open-Meteo adapter requires a Doer (inject a fake in tests; pass http.DefaultClient for live calls)")
	}
	if baseURL == "" {
		baseURL = DefaultOpenMeteoBaseURL
	}
	return &OpenMeteo{BaseURL: baseURL, Doer: doer, Timezone: "Asia/Shanghai"}, nil
}

// NewLiveOpenMeteo dials api.open-meteo.com. Do not call from unit tests.
func NewLiveOpenMeteo() (*OpenMeteo, error) {
	return NewOpenMeteo(http.DefaultClient, DefaultOpenMeteoBaseURL)
}

func (c *OpenMeteo) Snapshot(ctx context.Context, loc Location, at time.Time) (Snapshot, error) {
	if c == nil || c.Doer == nil {
		return Snapshot{}, fmt.Errorf("weather: Open-Meteo adapter is not configured")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	if loc.Label == "" && loc.Latitude == 0 && loc.Longitude == 0 {
		loc = DefaultLocation
	}
	if err := validLocation(loc); err != nil {
		return Snapshot{}, err
	}

	reqURL, err := c.forecastURL(loc)
	if err != nil {
		return Snapshot{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return Snapshot{}, fmt.Errorf("weather: build Open-Meteo request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "proactive-workbench-datasources/0.1")

	resp, err := c.Doer.Do(req)
	if err != nil {
		return Snapshot{}, fmt.Errorf("weather: Open-Meteo request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Snapshot{}, fmt.Errorf("weather: read Open-Meteo body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Snapshot{}, fmt.Errorf("weather: Open-Meteo HTTP %d", resp.StatusCode)
	}

	zone := time.UTC
	if c.Timezone != "" {
		if loaded, err := time.LoadLocation(c.Timezone); err == nil {
			zone = loaded
		}
	}
	snap, err := ParseOpenMeteoJSON(body, loc, zone, false, OpenMeteoSourceLabel)
	if err != nil {
		return Snapshot{}, err
	}
	if !at.IsZero() {
		if p, ok := snap.PointAt(at); ok {
			snap.Current = p
			snap.ObservedAt = at
		}
	}
	return snap, nil
}

func (c *OpenMeteo) forecastURL(loc Location) (string, error) {
	base := c.BaseURL
	if base == "" {
		base = DefaultOpenMeteoBaseURL
	}
	u, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("weather: invalid Open-Meteo base URL: %w", err)
	}
	u.Path = "/v1/forecast"
	q := u.Query()
	q.Set("latitude", formatCoord(loc.Latitude))
	q.Set("longitude", formatCoord(loc.Longitude))
	q.Set("current", "temperature_2m,weather_code,precipitation")
	q.Set("hourly", "temperature_2m,weather_code,precipitation_probability,precipitation")
	q.Set("forecast_days", "2")
	tz := c.Timezone
	if tz == "" {
		tz = "UTC"
	}
	q.Set("timezone", tz)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func formatCoord(v float64) string {
	return strconv.FormatFloat(v, 'f', 4, 64)
}
