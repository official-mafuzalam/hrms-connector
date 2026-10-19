package heartbeat

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"hrms-connector/internal/config"
	"hrms-connector/internal/key"
	"hrms-connector/internal/relay"
	"hrms-connector/internal/version"
)

type ConfigProvider interface {
	Snapshot() config.Config
}

type StatsProvider interface {
	Snapshot() relay.Stats
}

type Status struct {
	State         string     `json:"state"`
	LastOK        *time.Time `json:"last_ok,omitempty"`
	Company       string     `json:"company,omitempty"`
	Site          string     `json:"site,omitempty"`
	ConnectorID   string     `json:"connector_id,omitempty"`
	LatestVersion string     `json:"latest_version,omitempty"`
	MinVersion    string     `json:"min_version,omitempty"`
	DownloadURL   string     `json:"download_url,omitempty"`
	Error         string     `json:"error,omitempty"`
}

type flexString string

func (s *flexString) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*s = ""
		return nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err == nil {
		*s = flexString(value)
		return nil
	}
	var valueNumber json.Number
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&valueNumber); err == nil {
		*s = flexString(valueNumber.String())
		return nil
	}
	return errors.New("expected a string, number, or null")
}

type VerificationError struct {
	Kind       string `json:"kind"`
	StatusCode int    `json:"status,omitempty"`
	Details    string `json:"details"`
}

func (e *VerificationError) Error() string {
	return e.Display("en")
}

func (e *VerificationError) Message(language string) string {
	switch e.Kind {
	case "invalid_key":
		if language == "bn" {
			return "সংযোগ কীটি সঠিক নয়। সঠিকভাবে কপি হয়েছে কি না যাচাই করুন।"
		}
		return "The connection key is invalid. Check that it was copied correctly."
	case "revoked":
		if language == "bn" {
			return "সংযোগ কীটি বাতিল করা হয়েছে। নতুন কী পেতে HRMS প্রশাসকের সাথে যোগাযোগ করুন।"
		}
		return "The connection key has been revoked. Ask your HRMS administrator for a new key."
	case "unreachable":
		if language == "bn" {
			return "এইচআরএমএস সার্ভারে সংযোগ করা যায়নি। নেটওয়ার্ক সংযোগ পরীক্ষা করে আবার চেষ্টা করুন।"
		}
		return "Could not reach the HRMS server. Check the network connection and try again."
	default:
		if language == "bn" {
			return "এইচআরএমএস সার্ভার থেকে অপ্রত্যাশিত উত্তর এসেছে। আবার চেষ্টা করুন বা HRMS প্রশাসকের সাথে যোগাযোগ করুন।"
		}
		return "The HRMS server returned an unexpected response. Try again or contact your HRMS administrator."
	}
}

func (e *VerificationError) Display(language string) string {
	detailsLabel := "Details"
	if language == "bn" {
		detailsLabel = "বিস্তারিত"
	}
	if e.Details == "" {
		return e.Message(language)
	}
	return e.Message(language) + "\n" + detailsLabel + ": " + e.Details
}

func (e *VerificationError) DisplayBilingual() string {
	return e.Message("en") + "\n" + e.Message("bn") + "\nDetails: " + e.Details
}

type handshakeResponse struct {
	OK            bool       `json:"ok"`
	ConnectorID   flexString `json:"connector_id"`
	Company       string     `json:"company"`
	Site          string     `json:"site"`
	LatestVersion flexString `json:"latest_version"`
	MinVersion    flexString `json:"min_version"`
	DownloadURL   flexString `json:"download_url"`
	Error         flexString `json:"error"`
}

func (r *handshakeResponse) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	value, exists := fields["ok"]
	if !exists || string(value) == "null" {
		return fmt.Errorf("missing required response field %q", "ok")
	}
	if err := json.Unmarshal(fields["ok"], &r.OK); err != nil {
		return fmt.Errorf("invalid required response field %q: %w", "ok", err)
	}
	known := map[string]func(json.RawMessage) error{
		"connector_id":   func(value json.RawMessage) error { return json.Unmarshal(value, &r.ConnectorID) },
		"company":        func(value json.RawMessage) error { return json.Unmarshal(value, &r.Company) },
		"site":           func(value json.RawMessage) error { return json.Unmarshal(value, &r.Site) },
		"latest_version": func(value json.RawMessage) error { return json.Unmarshal(value, &r.LatestVersion) },
		"min_version":    func(value json.RawMessage) error { return json.Unmarshal(value, &r.MinVersion) },
		"download_url":   func(value json.RawMessage) error { return json.Unmarshal(value, &r.DownloadURL) },
		"error":          func(value json.RawMessage) error { return json.Unmarshal(value, &r.Error) },
	}
	for name, value := range fields {
		if name == "ok" {
			continue
		}
		decode, exists := known[name]
		if !exists {
			log.Printf("warning: ignoring unknown optional HRMS response field %q", name)
			continue
		}
		if err := decode(value); err != nil {
			log.Printf("warning: ignoring invalid optional HRMS response field %q: %v", name, err)
		}
	}
	return nil
}

type Heartbeater struct {
	config    ConfigProvider
	stats     StatsProvider
	mu        sync.RWMutex
	status    Status
	started   time.Time
	transMu   sync.Mutex
	transKey  string
	transport *http.Transport
}

func VerifyKey(ctx context.Context, cfg config.Config, value string) (Status, error) {
	copyConfig := cfg
	copyConfig.Key = value
	h := New(&fixedConfig{cfg: copyConfig}, nil)
	defer h.closeIdleConnections()
	if err := h.handshake(ctx, copyConfig); err != nil {
		return Status{}, err
	}
	status := h.Snapshot()
	if status.State == "invalid_key" || status.State == "revoked" {
		return status, &VerificationError{Kind: status.State, Details: status.State}
	}
	return status, nil
}

type fixedConfig struct{ cfg config.Config }

func (f *fixedConfig) Snapshot() config.Config { return f.cfg }

func New(cfg ConfigProvider, stats StatsProvider) *Heartbeater {
	return &Heartbeater{config: cfg, stats: stats, started: time.Now(), status: Status{State: "unreachable"}}
}

func (h *Heartbeater) Snapshot() Status {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.status
}

func (h *Heartbeater) Run(ctx context.Context) {
	defer h.closeIdleConnections()
	currentKey := ""
	handshakeDone := false
	backoff := time.Second
	nextTry := time.Now()
	for {
		cfg := h.config.Snapshot()
		if cfg.Key != currentKey {
			currentKey = cfg.Key
			handshakeDone = false
			backoff = time.Second
			nextTry = time.Now()
			h.set(Status{State: "unreachable"})
		}
		now := time.Now()
		if cfg.Key == "" {
			h.set(Status{State: "unreachable"})
			nextTry = now.Add(60 * time.Second)
		} else if !now.Before(nextTry) {
			if !handshakeDone {
				if err := h.handshake(ctx, cfg); err != nil {
					h.markError(err)
				}
				handshakeDone = h.Snapshot().State == "connected"
			} else {
				if err := h.sendHeartbeat(ctx, cfg); err != nil {
					h.markError(err)
				}
			}
			if h.Snapshot().State == "connected" {
				backoff = time.Second
				nextTry = time.Now().Add(60 * time.Second)
			} else {
				nextTry = time.Now().Add(backoff)
				backoff *= 2
				if backoff > 60*time.Second {
					backoff = 60 * time.Second
				}
			}
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (h *Heartbeater) handshake(ctx context.Context, cfg config.Config) error {
	conn, err := key.Parse(cfg.Key)
	if err != nil {
		return &VerificationError{Kind: "invalid_key", Details: err.Error()}
	}
	payload := map[string]string{
		"hostname": hostname(), "lan_ip": lanIP(), "os": runtime.GOOS, "version": version.Version,
	}
	status, body, err := h.post(ctx, cfg, conn, "/api/connector/handshake", payload)
	if err != nil {
		return err
	}
	var result handshakeResponse
	if err := decodeResponse(status, body, conn.Token, &result); err != nil {
		return err
	}
	if status == http.StatusUnauthorized {
		if result.Error != "invalid_key" && result.Error != "revoked" {
			return unexpectedResponse(status, body, conn.Token, "unauthorized response did not contain a recognized error")
		}
		h.set(Status{State: string(result.Error), Error: string(result.Error)})
		return nil
	}
	if status < 200 || status >= 300 || !result.OK {
		return unexpectedResponse(status, body, conn.Token, "handshake response was not successful")
	}
	if result.Company == "" || result.Site == "" {
		return unexpectedResponse(status, body, conn.Token, "handshake response omitted company or site")
	}
	now := time.Now()
	h.set(Status{
		State: "connected", LastOK: &now, Company: result.Company, Site: result.Site,
		ConnectorID: string(result.ConnectorID), LatestVersion: string(result.LatestVersion),
		MinVersion: string(result.MinVersion), DownloadURL: string(result.DownloadURL),
	})
	return nil
}

func (h *Heartbeater) sendHeartbeat(ctx context.Context, cfg config.Config) error {
	conn, err := key.Parse(cfg.Key)
	if err != nil {
		return &VerificationError{Kind: "invalid_key", Details: err.Error()}
	}
	stats := h.stats.Snapshot()
	devices := make([]map[string]string, 0, len(stats.Devices))
	for _, device := range stats.Devices {
		devices = append(devices, map[string]string{
			"sn": device.SN, "ip": device.IP, "last_seen": device.LastSeen.Format(time.RFC3339),
		})
	}
	sort.Slice(devices, func(i, j int) bool { return devices[i]["sn"] < devices[j]["sn"] })
	payload := map[string]any{
		"version": version.Version, "os": runtime.GOOS, "hostname": hostname(), "lan_ip": lanIP(),
		"port": cfg.Port, "uptime_s": int64(time.Since(h.started).Seconds()),
		"counters": map[string]uint64{
			"requests": stats.Requests, "errors": stats.Errors, "upstream_503": stats.Upstream503,
		},
		"devices": devices,
	}
	status, body, err := h.post(ctx, cfg, conn, "/api/connector/heartbeat", payload)
	if err != nil {
		return err
	}
	var result handshakeResponse
	if err := decodeResponse(status, body, conn.Token, &result); err != nil {
		return err
	}
	if status == http.StatusUnauthorized {
		if result.Error != "invalid_key" && result.Error != "revoked" {
			return unexpectedResponse(status, body, conn.Token, "unauthorized response did not contain a recognized error")
		}
		h.set(Status{State: string(result.Error), Error: string(result.Error)})
		return nil
	}
	if status < 200 || status >= 300 || !result.OK {
		return unexpectedResponse(status, body, conn.Token, "heartbeat response was not successful")
	}
	h.mu.Lock()
	h.status.State = "connected"
	now := time.Now()
	h.status.LastOK = &now
	h.status.Error = ""
	h.status.LatestVersion = string(result.LatestVersion)
	h.status.MinVersion = string(result.MinVersion)
	h.status.DownloadURL = string(result.DownloadURL)
	h.mu.Unlock()
	return nil
}

func (h *Heartbeater) post(ctx context.Context, cfg config.Config, conn *key.Connection, path string, input any) (int, []byte, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return 0, nil, err
	}
	target := *conn.URL
	target.Path += path
	reqCtx, cancel := context.WithTimeout(ctx, cfg.Timeout())
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, target.String(), bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Connector-Token", conn.Token)
	resp, err := h.client(cfg).Do(req)
	if err != nil {
		return 0, nil, &VerificationError{Kind: "unreachable", Details: redact(err.Error(), conn.Token)}
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024+1))
	if err != nil {
		return resp.StatusCode, nil, &VerificationError{Kind: "unreachable", StatusCode: resp.StatusCode, Details: redact(err.Error(), conn.Token)}
	}
	if len(responseBody) > 64*1024 {
		responseBody = responseBody[:64*1024]
	}
	return resp.StatusCode, responseBody, nil
}

func decodeResponse(status int, body []byte, token string, output *handshakeResponse) error {
	if status != http.StatusUnauthorized && (status < 200 || status >= 300) {
		return unexpectedResponse(status, body, token, "")
	}
	if err := json.Unmarshal(body, output); err != nil {
		return unexpectedResponse(status, body, token, "invalid JSON response: "+err.Error())
	}
	return nil
}

func unexpectedResponse(status int, body []byte, token, reason string) *VerificationError {
	details := fmt.Sprintf("HTTP %d: %s", status, responsePreview(body, token))
	if reason != "" {
		details += " (" + redact(reason, token) + ")"
	}
	return &VerificationError{Kind: "unexpected_response", StatusCode: status, Details: details}
}

func responsePreview(body []byte, token string) string {
	text := redact(string(body), token)
	text = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, text)
	text = strings.TrimSpace(text)
	if utf8.RuneCountInString(text) > 200 {
		runes := []rune(text)
		text = string(runes[:200])
	}
	if text == "" {
		return "<empty response>"
	}
	return text
}

func redact(value, token string) string {
	if token != "" {
		value = strings.ReplaceAll(value, token, "[redacted]")
	}
	return value
}

func (h *Heartbeater) client(cfg config.Config) *http.Client {
	transportKey := cfg.HTTPProxy + "|" + strconv.FormatBool(cfg.TLSVerify)
	h.transMu.Lock()
	defer h.transMu.Unlock()
	if h.transport == nil || h.transKey != transportKey {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: !cfg.TLSVerify} //nolint:gosec
		if cfg.HTTPProxy != "" {
			proxyURL, _ := url.Parse(cfg.HTTPProxy)
			transport.Proxy = http.ProxyURL(proxyURL)
		} else {
			transport.Proxy = http.ProxyFromEnvironment
		}
		if h.transport != nil {
			h.transport.CloseIdleConnections()
		}
		h.transport, h.transKey = transport, transportKey
	}
	return &http.Client{Transport: h.transport}
}

func (h *Heartbeater) closeIdleConnections() {
	h.transMu.Lock()
	defer h.transMu.Unlock()
	if h.transport != nil {
		h.transport.CloseIdleConnections()
	}
}

func (h *Heartbeater) set(status Status) {
	h.mu.Lock()
	h.status = status
	h.mu.Unlock()
}

func (h *Heartbeater) markError(err error) {
	h.mu.Lock()
	h.status.State = "unreachable"
	var verificationError *VerificationError
	if errors.As(err, &verificationError) {
		h.status.Error = verificationError.Details
	} else {
		h.status.Error = err.Error()
	}
	h.mu.Unlock()
}

func hostname() string {
	name, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return name
}

func lanIP() string {
	conn, err := net.Dial("udp", "192.0.2.1:80")
	if err == nil {
		defer conn.Close()
		if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok {
			return addr.IP.String()
		}
	}
	return ""
}
