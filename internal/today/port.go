package today

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

// DefaultListenAddr is the loopback address from PR #3 API.md.
const DefaultListenAddr = "127.0.0.1:8741"

const (
	portFileEnv      = "PW_PORT_FILE"
	darwinAppSupport = "Library/Application Support/Today Workbench"
	linuxConfigDir   = "today-workbench"
	portFileName     = "port"
)

// PortFilePath is where the process writes the address it actually bound.
//
//	macOS:  ~/Library/Application Support/Today Workbench/port
//	Linux:  ${XDG_CONFIG_HOME:-~/.config}/today-workbench/port
//
// Override with PW_PORT_FILE (used by tests and one-off QA).
func PortFilePath() string {
	return portFilePathFor(runtime.GOOS, os.Getenv("HOME"), os.Getenv("XDG_CONFIG_HOME"), os.Getenv(portFileEnv))
}

func portFilePathFor(goos, home, xdg, override string) string {
	if strings.TrimSpace(override) != "" {
		return override
	}
	if home == "" {
		if h, err := os.UserHomeDir(); err == nil {
			home = h
		}
	}
	if goos == "darwin" {
		return filepath.Join(home, darwinAppSupport, portFileName)
	}
	cfg := strings.TrimSpace(xdg)
	if cfg == "" {
		cfg = filepath.Join(home, ".config")
	}
	return filepath.Join(cfg, linuxConfigDir, portFileName)
}

// WritePortFile records host:port (one line) so QA can find a fallback bind.
func WritePortFile(host, port string) (string, error) {
	if host == "" || port == "" {
		return "", fmt.Errorf("today: host and port are required")
	}
	path := PortFilePath()
	if path == "" {
		return "", fmt.Errorf("today: empty port file path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	body := net.JoinHostPort(host, port) + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// ListenPreferred binds preferred. On EADDRINUSE it falls back to host:0.
func ListenPreferred(preferred string) (net.Listener, error) {
	if strings.TrimSpace(preferred) == "" {
		preferred = DefaultListenAddr
	}
	ln, err := net.Listen("tcp", preferred)
	if err == nil {
		return ln, nil
	}
	if !isAddrInUse(err) {
		return nil, err
	}
	host, _, splitErr := net.SplitHostPort(preferred)
	if splitErr != nil || host == "" {
		host = "127.0.0.1"
	}
	return net.Listen("tcp", net.JoinHostPort(host, "0"))
}

func isAddrInUse(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, syscall.EADDRINUSE) {
		return true
	}
	var op *net.OpError
	if errors.As(err, &op) && op.Err != nil {
		if errors.Is(op.Err, syscall.EADDRINUSE) {
			return true
		}
	}
	msg := err.Error()
	return strings.Contains(msg, "address already in use") || strings.Contains(msg, "bind: Only one usage")
}
