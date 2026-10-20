package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"hrms-connector/internal/config"
	"hrms-connector/internal/portcheck"
)

func TestWaitForURLWaitsUntilUIIsReady(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if attempts.Add(1) < 3 {
			http.Error(w, "starting", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	if err := waitForURL(server.URL, time.Second); err != nil {
		t.Fatal(err)
	}
	if attempts.Load() < 3 {
		t.Fatalf("wait-ready returned after %d attempts", attempts.Load())
	}
}

func TestWaitForURLTimesOutWhenPortIsUnavailable(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	err = waitForURL(fmt.Sprintf("http://127.0.0.1:%d/", port), 25*time.Millisecond)
	if err == nil {
		t.Fatal("expected wait-ready timeout")
	}
}

func TestWaitReadyFlagParsing(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
		valid bool
	}{
		{input: "30", want: 30 * time.Second, valid: true},
		{input: "30s", want: 30 * time.Second, valid: true},
		{input: "1m", want: time.Minute, valid: true},
		{input: "not-a-duration", valid: false},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			_, got, err := parseWaitReadyArgs([]string{"--timeout", test.input, "--data-dir", t.TempDir()})
			if test.valid {
				if err != nil || got != test.want {
					t.Fatalf("wait-ready timeout %q parsed as %s, %v; want %s", test.input, got, err, test.want)
				}
				return
			}
			if err == nil {
				t.Fatalf("wait-ready timeout %q unexpectedly succeeded: %s", test.input, got)
			}
		})
	}
}

func TestCheckPortCommandChecksBindAndDecimalInput(t *testing.T) {
	var listener net.Listener
	var busyPort int
	for candidate := 18085; candidate < 18185; candidate++ {
		current, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", candidate))
		if err == nil {
			listener, busyPort = current, candidate
			break
		}
	}
	if listener == nil {
		t.Fatal("could not reserve a test port in the allowed range")
	}
	if err := checkPort([]string{strconv.Itoa(busyPort)}); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("busy port check error = %v, want unavailable", err)
	}
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	if err := checkPort([]string{strconv.Itoa(busyPort)}); err != nil {
		t.Fatalf("freed port check failed: %v", err)
	}
	if err := checkPort([]string{"+8085"}); err == nil || !strings.Contains(err.Error(), "digits only") {
		t.Fatalf("signed port input error = %v, want digits-only error", err)
	}
	if err := checkPort([]string{"8085", "8088"}); err == nil {
		t.Fatal("expected check-port to reject multiple arguments")
	}
}

func TestFindAvailablePortPreservesPreferredAndSkipsBusyOrExcludedPorts(t *testing.T) {
	var occupiedListener net.Listener
	var occupiedPort int
	for candidate := 18000; candidate < 18100; candidate++ {
		listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", candidate))
		if err == nil {
			occupiedListener, occupiedPort = listener, candidate
			break
		}
	}
	if occupiedListener == nil {
		t.Fatal("could not reserve a test port")
	}
	defer occupiedListener.Close()

	selected, err := findAvailablePort(occupiedPort, 0, "0.0.0.0", portcheck.ExecRunner{})
	if err != nil {
		t.Fatal(err)
	}
	if selected == occupiedPort {
		t.Fatalf("selected occupied preferred port %d", occupiedPort)
	}

	if err := occupiedListener.Close(); err != nil {
		t.Fatal(err)
	}
	selected, err = findAvailablePort(occupiedPort, 0, "0.0.0.0", portcheck.ExecRunner{})
	if err != nil {
		t.Fatal(err)
	}
	if selected != occupiedPort {
		t.Fatalf("selected port %d, want preferred port %d", selected, occupiedPort)
	}

	selected, err = findAvailablePort(occupiedPort, occupiedPort, "0.0.0.0", portcheck.ExecRunner{})
	if err != nil {
		t.Fatal(err)
	}
	if selected == occupiedPort {
		t.Fatalf("selected excluded port %d", occupiedPort)
	}
}

func TestConfigPortOutputHasNoTrailingNewline(t *testing.T) {
	dataDir := t.TempDir()
	cfg := config.Default()
	cfg.Port, cfg.UIPort = 8086, 8087
	if err := config.Save(filepath.Join(dataDir, "config.json"), cfg); err != nil {
		t.Fatal(err)
	}

	originalStdout := os.Stdout
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writePipe
	callErr := showConfigPort([]string{"--kind", "relay", "--data-dir", dataDir})
	os.Stdout = originalStdout
	if closeErr := writePipe.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	output, readErr := io.ReadAll(readPipe)
	_ = readPipe.Close()
	if callErr != nil {
		t.Fatal(callErr)
	}
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(output) != "8086" {
		t.Fatalf("config-port output = %q, want exact numeric value without newline", output)
	}
}
