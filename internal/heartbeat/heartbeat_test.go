package heartbeat

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"hrms-connector/internal/config"
	"hrms-connector/internal/relay"
	"hrms-connector/internal/version"
)

type statsValue struct{ stats relay.Stats }

func (s *statsValue) Snapshot() relay.Stats { return s.stats }

func TestHandshakeAndHeartbeatContract(t *testing.T) {
	var handshakePayload, heartbeatPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Connector-Token") != "heartbeat-token" {
			t.Errorf("missing connector token header")
		}
		if r.Method != http.MethodPost {
			t.Errorf("method %s, want POST", r.Method)
		}
		if r.URL.Path == "/api/connector/handshake" {
			if err := json.NewDecoder(r.Body).Decode(&handshakePayload); err != nil {
				t.Errorf("decode handshake: %v", err)
			}
			_, _ = fmt.Fprint(w, `{"ok":true,"connector_id":"c-7","company":"Factory Co","site":"Dhaka","latest_version":"2.0.0","min_version":"1.0.0","download_url":"https://download.example/connector"}`)
			return
		}
		if r.URL.Path == "/api/connector/heartbeat" {
			if err := json.NewDecoder(r.Body).Decode(&heartbeatPayload); err != nil {
				t.Errorf("decode heartbeat: %v", err)
			}
			_, _ = fmt.Fprint(w, `{"ok":true,"latest_version":"2.0.0","min_version":"1.0.0","download_url":"https://download.example/connector"}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	cfg := config.Default()
	cfg.Key = "HRMS1." + encodePayload(fmt.Sprintf(`{"v":1,"u":%q,"t":"heartbeat-token"}`, server.URL))
	provider := &fixedConfig{cfg: cfg}
	stats := &statsValue{stats: relay.Stats{
		Counters: relay.Counters{Requests: 19, Errors: 2, Upstream503: 1},
		Devices:  []relay.Device{{SN: "serial-1", IP: "10.0.0.7", LastSeen: time.Now()}},
	}}
	h := New(provider, stats)
	if err := h.handshake(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	if err := h.sendHeartbeat(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	state := h.Snapshot()
	if state.State != "connected" || state.ConnectorID != "c-7" || state.Company != "Factory Co" || state.Site != "Dhaka" || state.LatestVersion != "2.0.0" {
		t.Fatalf("unexpected heartbeat state: %+v", state)
	}
	if handshakePayload["version"] != version.Version || handshakePayload["os"] == "" || handshakePayload["hostname"] == "" {
		t.Fatalf("incomplete handshake payload: %+v", handshakePayload)
	}
	if heartbeatPayload["version"] != version.Version || heartbeatPayload["port"] != float64(cfg.Port) || heartbeatPayload["uptime_s"] == nil {
		t.Fatalf("incomplete heartbeat payload: %+v", heartbeatPayload)
	}
	counters := heartbeatPayload["counters"].(map[string]any)
	if counters["requests"] != float64(19) || counters["errors"] != float64(2) || counters["upstream_503"] != float64(1) {
		t.Fatalf("incorrect counters: %+v", counters)
	}
	devices := heartbeatPayload["devices"].([]any)
	if len(devices) != 1 || devices[0].(map[string]any)["sn"] != "serial-1" {
		t.Fatalf("incorrect devices payload: %+v", devices)
	}
}

func encodePayload(value string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(value))
}

func verificationConfig(serverURL, token string) config.Config {
	cfg := config.Default()
	cfg.Key = "HRMS1." + encodePayload(fmt.Sprintf(`{"v":1,"u":%q,"t":%q}`, serverURL, token))
	return cfg
}

func TestVerifyKeyAcceptsFlexibleConnectorIDs(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want string
	}{
		{name: "number", id: `42`, want: "42"},
		{name: "string", id: `"c-7"`, want: "c-7"},
		{name: "null", id: `null`},
		{name: "missing"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := `{"ok":true,"company":"Factory Co","site":"Dhaka"`
			if test.id != "" {
				response += `,"connector_id":` + test.id
			}
			response += `}`
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = fmt.Fprint(w, response)
			}))
			defer server.Close()
			status, err := VerifyKey(context.Background(), verificationConfig(server.URL, "test-secret"), verificationConfig(server.URL, "test-secret").Key)
			if err != nil {
				t.Fatal(err)
			}
			if status.ConnectorID != test.want {
				t.Fatalf("connector_id=%q, want %q", status.ConnectorID, test.want)
			}
		})
	}
}

func TestVerifyKeyIgnoresUnknownAndWrongTypedOptionalFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"ok":true,"company":"Factory Co","site":"Dhaka","connector_id":{"unexpected":"object"},"latest_version":false,"future_optional":{"value":1}}`)
	}))
	defer server.Close()
	cfg := verificationConfig(server.URL, "test-secret")
	status, err := VerifyKey(context.Background(), cfg, cfg.Key)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "connected" || status.Company != "Factory Co" || status.Site != "Dhaka" || status.ConnectorID != "" || status.LatestVersion != "" {
		t.Fatalf("unexpected verification status: %+v", status)
	}
}

func TestVerifyKeyHandlesUnauthorizedErrors(t *testing.T) {
	for _, code := range []string{"invalid_key", "revoked"} {
		t.Run(code, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = fmt.Fprintf(w, `{"ok":false,"error":%q}`, code)
			}))
			defer server.Close()
			cfg := verificationConfig(server.URL, "test-secret")
			status, err := VerifyKey(context.Background(), cfg, cfg.Key)
			var verificationError *VerificationError
			if !errors.As(err, &verificationError) {
				t.Fatalf("expected VerificationError, got %v", err)
			}
			if status.State != code || verificationError.Kind != code || !strings.Contains(verificationError.Display("en"), "Details: "+code) {
				t.Fatalf("unexpected unauthorized result: status=%+v error=%+v", status, verificationError)
			}
			if !strings.Contains(verificationError.Display("bn"), "বিস্তারিত: "+code) {
				t.Fatalf("Bengali display omitted Details line: %s", verificationError.Display("bn"))
			}
		})
	}
}

func TestVerifyKeyReportsUnexpectedHTMLResponse(t *testing.T) {
	const token = "test-secret"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = fmt.Fprintf(w, "<html>upstream failed; token=%s %s</html>END-TAIL", token, strings.Repeat("x", 240))
	}))
	defer server.Close()
	cfg := verificationConfig(server.URL, token)
	_, err := VerifyKey(context.Background(), cfg, cfg.Key)
	var verificationError *VerificationError
	if !errors.As(err, &verificationError) {
		t.Fatalf("expected VerificationError, got %v", err)
	}
	if verificationError.Kind != "unexpected_response" || !strings.Contains(verificationError.Details, "HTTP 502") || !strings.Contains(verificationError.Details, "<html>upstream failed") {
		t.Fatalf("unexpected response details: %+v", verificationError)
	}
	if strings.Contains(verificationError.DisplayBilingual(), token) {
		t.Fatal("verification error exposed the connection token")
	}
	if strings.Contains(verificationError.Details, "END-TAIL") {
		t.Fatalf("response preview was not truncated: %s", verificationError.Details)
	}
}

func TestVerifyKeyReportsTimeoutAsUnreachable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_, _ = fmt.Fprint(w, `{"ok":true,"company":"Factory Co","site":"Dhaka"}`)
	}))
	defer server.Close()
	cfg := verificationConfig(server.URL, "test-secret")
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	_, err := VerifyKey(ctx, cfg, cfg.Key)
	var verificationError *VerificationError
	if !errors.As(err, &verificationError) {
		t.Fatalf("expected VerificationError, got %v", err)
	}
	if verificationError.Kind != "unreachable" || !strings.Contains(verificationError.Details, "context deadline exceeded") {
		t.Fatalf("unexpected timeout error: %+v", verificationError)
	}
}
