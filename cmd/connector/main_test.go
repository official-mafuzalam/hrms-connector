package main

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
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
