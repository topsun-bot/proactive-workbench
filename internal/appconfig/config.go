// Package appconfig loads the user config that feeds live weather and ICS.
//
// File location (same directory as last-run / port files):
//
//	macOS:  ~/Library/Application Support/Today Workbench/config.json
//	Linux:  $XDG_CONFIG_HOME/today-workbench/config.json
//	        (fallback ~/.config/today-workbench/config.json)
//
// Keys (JSON):
//
//	lat        float   Open-Meteo latitude (optional)
//	lon        float   Open-Meteo longitude (optional)
//	timezone   string  IANA timezone (optional; default Asia/Shanghai)
//	ics_path   string  ICS file or directory (optional)
//	debug_fixture bool opt into labeled fixtures (same as --debug-fixture)
//
// Environment overrides (win over the file): PW_LAT, PW_LON, PW_TIMEZONE,
// PW_ICS_PATH, PW_DEBUG_FIXTURE, PW_CONFIG (alternate file path).
//
// Missing lat/lon fall back to Shanghai Changning District. See
// weather.DefaultLocation for the cited coordinates. No GeoClue / CoreLocation.
package appconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/topsun-bot/proactive-workbench/internal/datasources/weather"
	"github.com/topsun-bot/proactive-workbench/internal/proactivity"
)

const (
	ConfigFileName  = "config.json"
	EnvConfig       = "PW_CONFIG"
	EnvLat          = "PW_LAT"
	EnvLon          = "PW_LON"
	EnvTimezone     = "PW_TIMEZONE"
	EnvICSPath      = "PW_ICS_PATH"
	EnvDebugFixture = "PW_DEBUG_FIXTURE"
	DefaultTimezone = "Asia/Shanghai"
)

// File is the on-disk JSON schema.
type File struct {
	Lat          *float64 `json:"lat"`
	Lon          *float64 `json:"lon"`
	Timezone     string   `json:"timezone"`
	ICSPath      string   `json:"ics_path"`
	DebugFixture bool     `json:"debug_fixture"`
}

// Config is the merged runtime config (file + env + defaults).
type Config struct {
	Lat                 float64
	Lon                 float64
	Timezone            string
	ICSPath             string
	DebugFixture        bool
	Path                string
	UsedDefaultLocation bool
	FileMissing         bool
}

// Location is the Open-Meteo query. Unset lat/lon use Changning defaults.
func (c Config) Location() weather.Location {
	if c.UsedDefaultLocation {
		return weather.DefaultLocation
	}
	return weather.Location{
		Latitude:  c.Lat,
		Longitude: c.Lon,
		Label:     fmt.Sprintf("configured %.4f,%.4f (not a live GPS fix)", c.Lat, c.Lon),
	}
}

// DefaultPath is the platform config.json path.
// goos and getenv are injectable so tests do not touch the real home directory.
func DefaultPath(goos string, getenv func(string) string) string {
	if goos == "" {
		goos = runtime.GOOS
	}
	if getenv == nil {
		getenv = os.Getenv
	}
	if override := strings.TrimSpace(getenv(EnvConfig)); override != "" {
		return override
	}
	home := getenv("HOME")
	if goos == "darwin" {
		return filepath.Join(home, "Library", "Application Support", proactivity.AppSupportNameMac, ConfigFileName)
	}
	xdg := getenv("XDG_CONFIG_HOME")
	if xdg == "" {
		xdg = filepath.Join(home, ".config")
	}
	return filepath.Join(xdg, proactivity.AppConfigNameXDG, ConfigFileName)
}

// Load reads path (or DefaultPath) and applies env overrides.
// A missing file is not an error: defaults are used.
func Load(path string, getenv func(string) string) (Config, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	if path == "" {
		path = DefaultPath("", getenv)
	}
	cfg := Config{
		Lat:                 weather.DefaultLocation.Latitude,
		Lon:                 weather.DefaultLocation.Longitude,
		Timezone:            DefaultTimezone,
		Path:                path,
		UsedDefaultLocation: true,
		FileMissing:         true,
	}

	body, err := os.ReadFile(path)
	if err == nil {
		cfg.FileMissing = false
		var f File
		if err := json.Unmarshal(body, &f); err != nil {
			return Config{}, fmt.Errorf("appconfig: parse %s: %w", path, err)
		}
		if strings.TrimSpace(f.Timezone) != "" {
			cfg.Timezone = strings.TrimSpace(f.Timezone)
		}
		cfg.ICSPath = strings.TrimSpace(f.ICSPath)
		cfg.DebugFixture = f.DebugFixture
		if f.Lat != nil && f.Lon != nil {
			cfg.Lat = *f.Lat
			cfg.Lon = *f.Lon
			cfg.UsedDefaultLocation = false
		}
	} else if !os.IsNotExist(err) {
		return Config{}, fmt.Errorf("appconfig: read %s: %w", path, err)
	}

	if raw := strings.TrimSpace(getenv(EnvTimezone)); raw != "" {
		cfg.Timezone = raw
	}
	if raw := strings.TrimSpace(getenv(EnvICSPath)); raw != "" {
		cfg.ICSPath = raw
	}
	if envTruthy(getenv(EnvDebugFixture)) {
		cfg.DebugFixture = true
	}
	latRaw := strings.TrimSpace(getenv(EnvLat))
	lonRaw := strings.TrimSpace(getenv(EnvLon))
	if latRaw != "" || lonRaw != "" {
		if latRaw == "" || lonRaw == "" {
			return Config{}, fmt.Errorf("appconfig: %s and %s must be set together", EnvLat, EnvLon)
		}
		lat, err := strconv.ParseFloat(latRaw, 64)
		if err != nil {
			return Config{}, fmt.Errorf("appconfig: %s: %w", EnvLat, err)
		}
		lon, err := strconv.ParseFloat(lonRaw, 64)
		if err != nil {
			return Config{}, fmt.Errorf("appconfig: %s: %w", EnvLon, err)
		}
		cfg.Lat = lat
		cfg.Lon = lon
		cfg.UsedDefaultLocation = false
	}
	if cfg.Lat < -90 || cfg.Lat > 90 {
		return Config{}, fmt.Errorf("appconfig: lat %v out of range", cfg.Lat)
	}
	if cfg.Lon < -180 || cfg.Lon > 180 {
		return Config{}, fmt.Errorf("appconfig: lon %v out of range", cfg.Lon)
	}
	return cfg, nil
}

func envTruthy(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" {
		return false
	}
	switch strings.ToLower(v) {
	case "0", "false", "off", "no":
		return false
	default:
		return true
	}
}
