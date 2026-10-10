package proactivity

import (
	"path/filepath"
	"testing"
)

func TestDefaultLastRunPathDarwin(t *testing.T) {
	getenv := func(k string) string {
		if k == "HOME" {
			return "/Users/shaoruru"
		}
		return ""
	}
	got := DefaultLastRunPath("darwin", getenv)
	want := filepath.Join("/Users/shaoruru", "Library", "Application Support", AppSupportNameMac, LastRunFileName)
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestDefaultLastRunPathLinuxXDG(t *testing.T) {
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
	got := DefaultLastRunPath("linux", getenv)
	want := filepath.Join("/home/a4/.xdg-config", AppConfigNameXDG, LastRunFileName)
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestDefaultLastRunPathLinuxFallback(t *testing.T) {
	getenv := func(k string) string {
		if k == "HOME" {
			return "/home/a4"
		}
		return ""
	}
	got := DefaultLastRunPath("linux", getenv)
	want := filepath.Join("/home/a4", ".config", AppConfigNameXDG, LastRunFileName)
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
