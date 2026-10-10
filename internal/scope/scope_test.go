package scope

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRainThreshold(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "scope.yaml")
	if err := os.WriteFile(path, []byte("rain_threshold_pct: 40  # assumed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RainThresholdPct != 40 {
		t.Fatalf("got %d", cfg.RainThresholdPct)
	}
}

func TestFindReadsRepoScope(t *testing.T) {
	cfg := Find()
	if cfg.RainThresholdPct <= 0 {
		t.Fatalf("threshold %d", cfg.RainThresholdPct)
	}
}
