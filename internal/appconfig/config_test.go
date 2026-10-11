package appconfig_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/topsun-bot/proactive-workbench/internal/appconfig"
	"github.com/topsun-bot/proactive-workbench/internal/datasources/weather"
	"github.com/topsun-bot/proactive-workbench/internal/proactivity"
)

func TestDefaultPathDarwin(t *testing.T) {
	getenv := func(k string) string {
		if k == "HOME" {
			return "/Users/shaoruru"
		}
		return ""
	}
	got := appconfig.DefaultPath("darwin", getenv)
	want := filepath.Join("/Users/shaoruru", "Library", "Application Support", proactivity.AppSupportNameMac, appconfig.ConfigFileName)
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestDefaultPathLinuxXDG(t *testing.T) {
	getenv := func(k string) string {
		switch k {
		case "HOME":
			return "/home/a4"
		case "XDG_CONFIG_HOME":
			return "/home/a4/.xdg-config"
		default:
			return ""
		}
	}
	got := appconfig.DefaultPath("linux", getenv)
	want := filepath.Join("/home/a4/.xdg-config", proactivity.AppConfigNameXDG, appconfig.ConfigFileName)
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestLoadMissingUsesChangning(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")
	cfg, err := appconfig.Load(path, func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.UsedDefaultLocation || !cfg.FileMissing {
		t.Fatalf("%#v", cfg)
	}
	loc := cfg.Location()
	if loc.Latitude != weather.DefaultLocation.Latitude || loc.Longitude != weather.DefaultLocation.Longitude {
		t.Fatalf("location %#v", loc)
	}
	if loc.Latitude != 31.2205 || loc.Longitude != 121.4248 {
		t.Fatalf("Changning coords drifted: %#v", loc)
	}
}

func TestLoadFileAndEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	lat, lon := 31.2, 121.4
	body, err := json.Marshal(map[string]any{
		"lat": lat, "lon": lon, "timezone": "Asia/Tokyo", "ics_path": "/tmp/cal.ics",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := appconfig.Load(path, func(k string) string {
		if k == appconfig.EnvICSPath {
			return "/override/calendar"
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UsedDefaultLocation || cfg.Lat != lat || cfg.ICSPath != "/override/calendar" {
		t.Fatalf("%#v", cfg)
	}
	if cfg.Timezone != "Asia/Tokyo" {
		t.Fatalf("tz %q", cfg.Timezone)
	}
}
