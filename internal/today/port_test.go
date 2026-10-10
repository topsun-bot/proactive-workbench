package today

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPortFilePathOverride(t *testing.T) {
	t.Setenv("PW_PORT_FILE", "/tmp/qa-port")
	if got := PortFilePath(); got != "/tmp/qa-port" {
		t.Fatalf("PortFilePath() = %q", got)
	}
}

func TestPortFilePathForDarwinAndLinux(t *testing.T) {
	darwin := portFilePathFor("darwin", "/Users/ada", "", "")
	wantDarwin := "/Users/ada/Library/Application Support/Today Workbench/port"
	if darwin != wantDarwin {
		t.Fatalf("darwin path = %q want %q", darwin, wantDarwin)
	}
	linuxXDG := portFilePathFor("linux", "/home/ada", "/xdg/cfg", "")
	if linuxXDG != "/xdg/cfg/today-workbench/port" {
		t.Fatalf("linux xdg path = %q", linuxXDG)
	}
	linuxHome := portFilePathFor("linux", "/home/ada", "", "")
	if linuxHome != "/home/ada/.config/today-workbench/port" {
		t.Fatalf("linux home path = %q", linuxHome)
	}
	if portFilePathFor("linux", "/home/ada", "", "/forced/port") != "/forced/port" {
		t.Fatal("override should win")
	}
}

func TestWritePortFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Today Workbench", "port")
	t.Setenv("PW_PORT_FILE", path)
	got, err := WritePortFile("127.0.0.1", "8741")
	if err != nil {
		t.Fatal(err)
	}
	if got != path {
		t.Fatalf("wrote %q want %q", got, path)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "127.0.0.1:8741\n" {
		t.Fatalf("body = %q", body)
	}
}

func TestListenPreferredFallsBackAndWritesActualPort(t *testing.T) {
	blocker, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = blocker.Close() })
	busy := blocker.Addr().String()

	dir := t.TempDir()
	t.Setenv("PW_PORT_FILE", filepath.Join(dir, "port"))

	url, srv, portPath, err := ListenAndServe(busy, time.Time{}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	if !strings.HasPrefix(url, "http://127.0.0.1:") {
		t.Fatalf("url %q", url)
	}
	body, err := os.ReadFile(portPath)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(string(body))
	if got == busy {
		t.Fatalf("expected fallback away from busy %s, file=%s url=%s", busy, got, url)
	}
	if !strings.Contains(url, got) {
		t.Fatalf("url %q should include port file %q", url, got)
	}
}

func TestListenPreferredUsesFreeAddr(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PW_PORT_FILE", filepath.Join(dir, "port"))
	url, srv, portPath, err := ListenAndServe("127.0.0.1:0", time.Time{}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	body, err := os.ReadFile(portPath)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(string(body))
	if !strings.HasPrefix(got, "127.0.0.1:") {
		t.Fatalf("port file %q", got)
	}
	if !strings.Contains(url, got) {
		t.Fatalf("url %q vs file %q", url, got)
	}
}

func TestDefaultListenAddrMatchesPR3(t *testing.T) {
	if DefaultListenAddr != "127.0.0.1:8741" {
		t.Fatalf("DefaultListenAddr = %q", DefaultListenAddr)
	}
}
