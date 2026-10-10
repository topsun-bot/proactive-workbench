package scope

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const DefaultRainThresholdPct = 50

// Config is the machine-readable slice of docs/PRD.md / config/scope.yaml
// that the umbrella flow actually consults.
type Config struct {
	RainThresholdPct int
	Source           string
}

func Default() Config {
	return Config{RainThresholdPct: DefaultRainThresholdPct, Source: "default"}
}

// Load reads rain_threshold_pct from a scope YAML without a YAML dependency.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	cfg := Default()
	cfg.Source = path
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if strings.TrimSpace(key) != "rain_threshold_pct" {
			continue
		}
		val = strings.TrimSpace(val)
		if i := strings.IndexAny(val, " \t#"); i >= 0 {
			val = val[:i]
		}
		n, err := strconv.Atoi(val)
		if err != nil {
			return Config{}, err
		}
		cfg.RainThresholdPct = n
	}
	return cfg, nil
}

// Find walks from cwd (and optional overrides) to locate config/scope.yaml.
func Find() Config {
	if env := os.Getenv("PW_SCOPE_FILE"); env != "" {
		if cfg, err := Load(env); err == nil {
			return cfg
		}
	}
	wd, err := os.Getwd()
	if err != nil {
		return Default()
	}
	dir := wd
	for i := 0; i < 6; i++ {
		candidate := filepath.Join(dir, "config", "scope.yaml")
		if cfg, err := Load(candidate); err == nil {
			return cfg
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return Default()
}
