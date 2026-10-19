package webui

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
	"hrms-connector/internal/config"
	"hrms-connector/internal/heartbeat"
	"hrms-connector/internal/logging"
	"hrms-connector/internal/netcfg"
	"hrms-connector/internal/relay"
)

type heartbeatValue struct{}

func (heartbeatValue) Snapshot() heartbeat.Status {
	return heartbeat.Status{State: "connected", Company: "Example"}
}

type setupStatsValue struct{ stats relay.Stats }

func (s setupStatsValue) Snapshot() relay.Stats { return s.stats }

type setupNetworkValue struct{ network netcfg.Network }

func (n setupNetworkValue) Status(context.Context) (netcfg.Network, error) { return n.network, nil }
func (n setupNetworkValue) StatusForDevices(context.Context, []string) (netcfg.Network, error) {
	return n.network, nil
}
func (n setupNetworkValue) SelectAdapter(context.Context, string) error { return nil }
func (n setupNetworkValue) Apply(context.Context, bool) (netcfg.Network, error) {
	return n.network, nil
}

func TestHostCSRFAndPINProtection(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.Key = "HRMS1." + base64.RawURLEncoding.EncodeToString([]byte(`{"v":1,"u":"https://hrms.example","t":"secret-token-value"}`))
	hash, err := bcrypt.GenerateFromPassword([]byte("7391"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	cfg.PINHash = string(hash)
	store := config.NewStore(filepath.Join(dir, "config.json"), cfg)
	logger, err := logging.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	ui := New(store, heartbeatValue{}, relay.New(store, nil), logger, dir)

	blocked := httptest.NewRequest(http.MethodGet, "http://evil.example/", nil)
	blocked.Host = "evil.example"
	blockedRecorder := httptest.NewRecorder()
	ui.ServeHTTP(blockedRecorder, blocked)
	if blockedRecorder.Code != http.StatusForbidden {
		t.Fatalf("untrusted host status=%d", blockedRecorder.Code)
	}

	sessionReq := httptest.NewRequest(http.MethodGet, "http://localhost:17082/api/session", nil)
	sessionReq.Host = "localhost:17082"
	sessionRec := httptest.NewRecorder()
	ui.ServeHTTP(sessionRec, sessionReq)
	var session struct {
		CSRF string `json:"csrf"`
	}
	if err := json.Unmarshal(sessionRec.Body.Bytes(), &session); err != nil || session.CSRF == "" {
		t.Fatalf("session token unavailable: %v body=%s", err, sessionRec.Body.String())
	}
	cookies := sessionRec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("session cookie missing: %+v", cookies)
	}
	post := func(origin, csrf, pin string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, "http://localhost:17082/api/check-port", bytes.NewBufferString(`{"port":17081}`))
		request.Host = "localhost:17082"
		request.Header.Set("Origin", origin)
		request.Header.Set("X-CSRF-Token", csrf)
		request.Header.Set("X-UI-PIN", pin)
		request.AddCookie(cookies[0])
		recorder := httptest.NewRecorder()
		ui.ServeHTTP(recorder, request)
		return recorder
	}
	if result := post("http://attacker.example", session.CSRF, "7391"); result.Code != http.StatusForbidden {
		t.Fatalf("cross-origin POST status=%d", result.Code)
	}
	if result := post("http://localhost:17082", "", "7391"); result.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF status=%d", result.Code)
	}
	if result := post("http://localhost:17082", session.CSRF, "wrong"); result.Code != http.StatusUnauthorized {
		t.Fatalf("wrong PIN status=%d", result.Code)
	}
	if result := post("http://localhost:17082", session.CSRF, "7391"); result.Code != http.StatusOK {
		t.Fatalf("valid POST status=%d body=%s", result.Code, result.Body.String())
	}
	for i := 0; i < 6; i++ {
		result := post("http://localhost:17082", session.CSRF, "wrong")
		if i < 5 && result.Code != http.StatusUnauthorized {
			t.Fatalf("wrong PIN attempt %d status=%d", i+1, result.Code)
		}
		if i == 5 && result.Code != http.StatusTooManyRequests {
			t.Fatalf("rate limit status=%d", result.Code)
		}
	}
}

func TestUIStatusDoesNotExposeConnectionToken(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.Key = "HRMS1." + base64.RawURLEncoding.EncodeToString([]byte(`{"v":1,"u":"https://hrms.example","t":"very-secret-token"}`))
	store := config.NewStore(filepath.Join(dir, "config.json"), cfg)
	logger, err := logging.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	ui := New(store, heartbeatValue{}, relay.New(store, nil), logger, dir)
	request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:17082/api/status", nil)
	request.Host = "127.0.0.1:17082"
	recorder := httptest.NewRecorder()
	ui.ServeHTTP(recorder, request)
	if strings.Contains(recorder.Body.String(), "very-secret-token") {
		t.Fatal("status API exposed connection token")
	}
	if !strings.Contains(recorder.Body.String(), "key_masked") {
		t.Fatal("status API omitted masked key")
	}
}

func TestVerifyKeyReturnsLocalizedFriendlyErrorAndDetails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprint(w, `{"ok":false,"error":"invalid_key"}`)
	}))
	defer server.Close()
	tests := []struct {
		language string
		message  string
	}{
		{language: "en", message: "The connection key is invalid."},
		{language: "bn", message: "সংযোগ কীটি সঠিক নয়।"},
	}
	for _, test := range tests {
		t.Run(test.language, func(t *testing.T) {
			dir := t.TempDir()
			cfg := config.Default()
			cfg.Language = test.language
			store := config.NewStore(filepath.Join(dir, "config.json"), cfg)
			logger, err := logging.New(dir)
			if err != nil {
				t.Fatal(err)
			}
			ui := New(store, heartbeatValue{}, relay.New(store, nil), logger, dir)
			keyValue := "HRMS1." + base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"v":1,"u":%q,"t":"verify-secret"}`, server.URL)))
			request := httptest.NewRequest(http.MethodPost, "/api/verify-key", strings.NewReader(fmt.Sprintf(`{"key":%q}`, keyValue)))
			recorder := httptest.NewRecorder()
			ui.verifyKey(recorder, request)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("verification status=%d body=%s", recorder.Code, recorder.Body.String())
			}
			var response struct {
				Message   string `json:"message"`
				MessageEN string `json:"message_en"`
				MessageBN string `json:"message_bn"`
				Details   string `json:"details"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(response.Message, test.message) || response.Details != "invalid_key" ||
				response.MessageEN == "" || response.MessageBN == "" {
				t.Fatalf("unexpected localized verification response: %+v", response)
			}
		})
	}
}

func TestSetupPageRedirectAndSetupData(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.Key = "HRMS1." + base64.RawURLEncoding.EncodeToString([]byte(`{"v":1,"u":"https://hrms.example","t":"secret"}`))
	store := config.NewStore(filepath.Join(dir, "config.json"), cfg)
	logger, err := logging.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	lastSeen := time.Now().Add(-time.Minute).UTC().Truncate(time.Second)
	stats := setupStatsValue{stats: relay.Stats{Devices: []relay.Device{{SN: "SN-100", IP: "192.168.40.50", LastSeen: lastSeen, Count: 8}}}}
	network := setupNetworkValue{network: netcfg.Network{
		Interface: "Ethernet", IP: "192.168.40.27", Prefix: 32, Gateway: "192.168.40.1",
		DNS: []string{"192.168.40.2"}, SubnetMask: "255.255.255.255",
	}}
	ui := New(store, heartbeatValue{}, stats, logger, dir, network)
	request := httptest.NewRequest(http.MethodGet, "http://localhost/setup", nil)
	request.Host = "localhost"
	recorder := httptest.NewRecorder()
	ui.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `id="page-setup"`) {
		t.Fatalf("setup page response=%d", recorder.Code)
	}
	dataRequest := httptest.NewRequest(http.MethodGet, "http://localhost/api/setup", nil)
	dataRecorder := httptest.NewRecorder()
	ui.setupData(dataRecorder, dataRequest)
	var response struct {
		Port            int              `json:"port"`
		UIPort          int              `json:"ui_port"`
		SelectedAddress string           `json:"selected_address"`
		Network         netcfg.Network   `json:"network"`
		Device          *relay.Device    `json:"latest_device"`
		Heartbeat       heartbeat.Status `json:"heartbeat"`
	}
	if err := json.Unmarshal(dataRecorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Port != 17081 || response.UIPort != 17082 || response.SelectedAddress != "192.168.40.27" {
		t.Fatalf("setup device port/address data mismatch: %+v", response)
	}
	if response.Network.Gateway != "192.168.40.1" || response.Device == nil || response.Device.SN != "SN-100" || response.Device.LastSeen.IsZero() {
		t.Fatalf("setup device live/network data missing: %+v", response)
	}
	if response.Heartbeat.Company != "Example" {
		t.Fatalf("setup HRMS company missing: %+v", response.Heartbeat)
	}
}

func TestFirstRunRootRedirectsToSetup(t *testing.T) {
	dir := t.TempDir()
	store := config.NewStore(filepath.Join(dir, "config.json"), config.Default())
	logger, err := logging.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	ui := New(store, heartbeatValue{}, relay.New(store, nil), logger, dir)
	request := httptest.NewRequest(http.MethodGet, "http://localhost/", nil)
	request.Host = "localhost"
	recorder := httptest.NewRecorder()
	ui.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusSeeOther || recorder.Header().Get("Location") != "/setup" {
		t.Fatalf("first-run redirect = %d %q", recorder.Code, recorder.Header().Get("Location"))
	}
}

func TestSuggestUnusedIPsUsesBestEffortProbe(t *testing.T) {
	network := netcfg.Network{IP: "192.168.40.27", Prefix: 24, Gateway: "192.168.40.1"}
	var probed []string
	addresses, err := suggestUnusedIPs(context.Background(), network, func(_ context.Context, address string) (bool, error) {
		probed = append(probed, address)
		return address == "192.168.40.2", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(addresses) != 3 || addresses[0] != "192.168.40.3" || len(probed) != 4 {
		t.Fatalf("unexpected suggestions %v after probes %v", addresses, probed)
	}
}
