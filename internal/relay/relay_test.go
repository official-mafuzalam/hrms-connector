package relay

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"hrms-connector/internal/config"
)

type configValue struct{ cfg config.Config }

func (c *configValue) Snapshot() config.Config { return c.cfg }

func encodedKey(serverURL string) string {
	payload := fmt.Sprintf(`{"v":1,"u":%q,"t":"device-token-secret"}`, serverURL)
	return "HRMS1." + base64.RawURLEncoding.EncodeToString([]byte(payload))
}

func testRequest(path string, body []byte) *http.Request {
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.RemoteAddr = "127.0.0.1:54321"
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	return req
}

func TestRelayPassesThroughAndTracksDevice(t *testing.T) {
	requestBody := []byte{0, 1, 2, '\n', 255}
	var upstreamPath, upstreamType, token, forwarded string
	var upstreamBody []byte
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.RequestURI()
		upstreamType = r.Header.Get("Content-Type")
		token = r.Header.Get("X-Connector-Token")
		forwarded = r.Header.Get("X-Forwarded-For")
		upstreamBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("OK\x00verbatim"))
	}))
	defer upstream.Close()
	cfg := config.Default()
	cfg.Key = encodedKey(upstream.URL + "/gateway")
	proxy := New(&configValue{cfg: cfg}, nil)
	recorder := httptest.NewRecorder()
	proxy.ServeHTTP(recorder, testRequest("/iclock/cdata%2Fbatch?SN=terminal-7&table=ATTLOG", requestBody))
	if recorder.Code != http.StatusAccepted || recorder.Body.String() != "OK\x00verbatim" {
		t.Fatalf("response changed: status=%d body=%q", recorder.Code, recorder.Body.Bytes())
	}
	if recorder.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Fatalf("content type changed: %q", recorder.Header().Get("Content-Type"))
	}
	if upstreamPath != "/gateway/iclock/cdata%2Fbatch?SN=terminal-7&table=ATTLOG" {
		t.Fatalf("upstream path/query = %q", upstreamPath)
	}
	if upstreamType != "text/plain; charset=utf-8" || !bytes.Equal(upstreamBody, requestBody) {
		t.Fatalf("request body/type changed: type=%q body=%v", upstreamType, upstreamBody)
	}
	if token != "device-token-secret" || forwarded != "127.0.0.1" {
		t.Fatalf("missing connector headers: token=%q forwarded=%q", token, forwarded)
	}
	stats := proxy.Snapshot()
	if stats.Requests != 1 || len(stats.Devices) != 1 || stats.Devices[0].SN != "terminal-7" || stats.Devices[0].Count != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}

func TestRelayForbiddenAndTooLarge(t *testing.T) {
	cfg := config.Default()
	cfg.AllowedCIDRs = []string{"192.168.0.0/16"}
	proxy := New(&configValue{cfg: cfg}, nil)
	req := testRequest("/iclock/cdata?SN=denied", []byte("body"))
	req.RemoteAddr = "203.0.113.9:1234"
	recorder := httptest.NewRecorder()
	proxy.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusForbidden || len(proxy.Snapshot().Devices) != 0 {
		t.Fatalf("forbidden request was not rejected cleanly: status=%d stats=%+v", recorder.Code, proxy.Snapshot())
	}

	cfg = config.Default()
	cfg.MaxBodyKB = 1
	proxy = New(&configValue{cfg: cfg}, nil)
	recorder = httptest.NewRecorder()
	proxy.ServeHTTP(recorder, testRequest("/iclock/cdata", bytes.Repeat([]byte("x"), 1025)))
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized body status=%d", recorder.Code)
	}
}

func TestRelayUpstreamUnavailableReturns503(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	upstreamURL := upstream.URL
	upstream.Close()
	cfg := config.Default()
	cfg.Key = encodedKey(upstreamURL)
	proxy := New(&configValue{cfg: cfg}, nil)
	recorder := httptest.NewRecorder()
	proxy.ServeHTTP(recorder, testRequest("/iclock/cdata?SN=retry-me", nil))
	if recorder.Code != http.StatusServiceUnavailable || !strings.Contains(recorder.Header().Get("Content-Type"), "text/plain") {
		t.Fatalf("upstream failure response status=%d type=%q", recorder.Code, recorder.Header().Get("Content-Type"))
	}
	stats := proxy.Snapshot()
	if stats.Errors != 1 || stats.Upstream503 != 1 {
		t.Fatalf("upstream failure counters = %+v", stats.Counters)
	}
}

func TestRelayThousandRequestsAndRSS(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("OK"))
	}))
	defer upstream.Close()
	cfg := config.Default()
	cfg.Key = encodedKey(upstream.URL)
	proxy := New(&configValue{cfg: cfg}, nil)
	relayServer := httptest.NewServer(proxy)
	defer relayServer.Close()
	client := relayServer.Client()
	for i := 0; i < 1000; i++ {
		response, err := client.Post(relayServer.URL+"/iclock/cdata?SN=terminal-1", "text/plain", strings.NewReader("attendance"))
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, response.Body)
		response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("request %d returned %d", i, response.StatusCode)
		}
	}
	if got := proxy.Snapshot().Requests; got != 1000 {
		t.Fatalf("recorded %d requests, want 1000", got)
	}
	if runtime.GOOS == "linux" {
		runtime.GC()
		data, err := os.ReadFile("/proc/self/status")
		if err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(line, "VmRSS:") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						kb, _ := strconv.Atoi(fields[1])
						t.Logf("RSS after 1000 relayed requests: %d KB", kb)
					}
					break
				}
			}
		}
	}
}
