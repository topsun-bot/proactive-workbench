package proactivity

import (
	"os"
	"path/filepath"
	"runtime"
)

const (
	// AppSupportNameMac is the macOS Application Support folder. Shaoruru’s
	// client is "Today Workbench" (PR #5 macos/).
	AppSupportNameMac = "Today Workbench"
	// AppConfigNameXDG is the Linux XDG config subdirectory (kebab-case).
	AppConfigNameXDG = "today-workbench"
	LastRunFileName  = "routines-last-run.json"
)

// DefaultLastRunPath returns the platform last-run file.
//
//	macOS:  ~/Library/Application Support/Today Workbench/routines-last-run.json
//	Linux:  $XDG_CONFIG_HOME/today-workbench/routines-last-run.json
//	        (fallback ~/.config/today-workbench/…)
//
// goos and getenv are injectable so tests do not touch the real home directory.
func DefaultLastRunPath(goos string, getenv func(string) string) string {
	if goos == "" {
		goos = runtime.GOOS
	}
	if getenv == nil {
		getenv = os.Getenv
	}
	home := getenv("HOME")
	if goos == "darwin" {
		return filepath.Join(home, "Library", "Application Support", AppSupportNameMac, LastRunFileName)
	}
	xdg := getenv("XDG_CONFIG_HOME")
	if xdg == "" {
		xdg = filepath.Join(home, ".config")
	}
	return filepath.Join(xdg, AppConfigNameXDG, LastRunFileName)
}
